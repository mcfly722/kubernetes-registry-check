package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"crypto/tls"
	"strings"
	"errors"
	"time"
	"net"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Pod struct {
	PodName  string
	PodIP    string
	HostName string
	HostIP   string
}

func (pod Pod) hash() string {
	s, _ := json.Marshal(pod)
	return string(s)
}

type RegistryConnectionResultRecord struct {
	Source  Pod
	Url     string
	Message string
	Success bool
}

func (record *RegistryConnectionResultRecord) toString() string {
	json, _ := json.Marshal(record)
	return string(json)
}

type RegistryAuth struct {
	Auth     string
	UserName string
	Password string
}

type RegistryAuths struct {
	Auths map[string]RegistryAuth
}

type Registry struct {
	Name     string
	Url      string
	UserName string
	Password string
}

type RegistryChecker struct {
	Done chan struct{}
}

type RegistryError struct {
	Code    string
	Message string
	Detail  string
}

type RegistryRepositories struct {
	Repositories []string
	Errors       []RegistryError
}

func (checker *RegistryChecker) Destroy() {
	close(checker.Done)
}

type k8s struct {
	clientset kubernetes.Interface
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func newPod(podName string, podIP string, hostName string, hostIP string) *Pod {
	return &Pod{
		PodName:  podName,
		PodIP:    podIP,
		HostName: hostName,
		HostIP:   hostIP,
	}
}

func newK8s() (*k8s, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client := k8s{}

	client.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func getRegistries(k8s *k8s, namespace string) (map[string]*Registry, error) {

	registries := map[string]*Registry{}

	secrets, err := k8s.clientset.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets.Items {

		b64data, ok := secret.Data[".dockerconfigjson"]
		if ok {
			var auths RegistryAuths

			err := json.Unmarshal(b64data, &auths)
			if err != nil {
				fmt.Println("decoding %v error:", secret.ObjectMeta.Name, err)
			} else {

				for key := range auths.Auths {
					_, keyIsAlreadyExists := registries[key]

					if !keyIsAlreadyExists {
						registries[key] = &Registry{
							Name:     secret.ObjectMeta.Name,
							Url:      key,
							UserName: auths.Auths[key].UserName,
							Password: auths.Auths[key].Password,
						}
					}

				}
			}
		}
	}

	return registries, nil
}

func checkRegistry(source *Pod, url string, userName string, password string) *RegistryConnectionResultRecord {

	result := &RegistryConnectionResultRecord{
		Source : *source, 
		Url    : url,
		Success: false,
	}

	fullUrl := fmt.Sprintf("https://%v/v2/_catalog", url)

	transport := &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }
	
    client := &http.Client{Transport: transport}
	
	request, err := http.NewRequest("GET", fullUrl, nil)
	if err != nil {
		result.Message = err.Error()
		return result
	}

	request.SetBasicAuth(userName, password)
	
	response, err := client.Do(request)
	if err != nil {
		result.Message = err.Error()
		return result
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		result.Message = err.Error()
		return result
	}

	var registryRepositories RegistryRepositories

	err = json.Unmarshal(body, &registryRepositories)
	if err != nil {
		result.Message = fmt.Sprintf("%v content:%v", err.Error(), string(body))
		return result
	}

	if len(registryRepositories.Errors) > 0 {
		result.Message = string(body)
		return result
	}

	result.Message = string(body)
	result.Success = true
	
	return result
}

func newRegistryChecker(sourcePod *Pod, registry *Registry, intervalSec int, output chan RegistryConnectionResultRecord, checkRegistry func(sourcePod *Pod, url string, userName string, password string) *RegistryConnectionResultRecord) (*RegistryChecker, error) {

	checker := RegistryChecker{
		Done: make(chan struct{}),
	}

	go func() {
	working:
		for {
			select {
			case <-checker.Done:
				break working
			default:
				{
					record := checkRegistry(sourcePod, registry.Url, registry.UserName, registry.Password)

					output <- *record

					time.Sleep(time.Duration(intervalSec) * time.Second)

				}
			}
		}
		fmt.Println(fmt.Sprintf("checker for '%v' registry has finished", registry.Name))
	}()

	return &checker, nil
}

func getPods(k8s *k8s, namespace string, podPrefix string) (map[string]Pod, error) {
	result := make(map[string]Pod)

	pods, err := k8s.clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if strings.HasPrefix(strings.ToUpper(pod.GetName()), strings.ToUpper(podPrefix)) {

			if pod.Status.Phase == "Running" {
				pod := Pod{
					PodName:  pod.GetName(),
					PodIP:    pod.Status.PodIP,
					HostName: pod.Spec.NodeName,
					HostIP:   pod.Status.HostIP}

				result[pod.hash()] = pod
			}
		}
	}

	return result, nil
}

func getUsedIPs() ([]string, error) {
	ips := []string{}

	ifaces, err := net.Interfaces()

	if err != nil {
		return nil, err
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()

		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ips = append(ips, fmt.Sprintf("%v", ip))
		}
	}

	return ips, nil
}


func getSourcePod(k8s *k8s, namespace string, podsPrefix string) (*Pod, error){
	ips, err := getUsedIPs()
	if err != nil {
		return nil, err
	}

	pods, err := getPods(k8s, namespace, podsPrefix)
	if err != nil {
		return nil, err
	}

	for _, pod := range pods { 
		if contains(ips, pod.PodIP) {
			return &pod, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("Could not found pod with '%v' prefix in '%v' namespace with eny of next ips: %v", podsPrefix, namespace, ips))
}

func newRegistryPool(k8s *k8s, namespace string, podsPrefix string, output chan RegistryConnectionResultRecord, configRefreshInterval time.Duration, checkIntervalSec int, checkRegistry func(sourcePod *Pod, url string, userName string, password string) *RegistryConnectionResultRecord) {

	sourcePod, err := getSourcePod(k8s, namespace, podsPrefix)
	if err != nil {
		panic(err)
	}

	checkers := map[string]*RegistryChecker{}

	for {
		registries, err := getRegistries(k8s, namespace)
		if err != nil {
			fmt.Println(fmt.Sprintf("error: %v", err))
		} else {

			for key, registry := range registries {
				_, registryIsAlreadyChecking := checkers[key]
				if !registryIsAlreadyChecking {
					// add new registry check

					checker, err := newRegistryChecker(sourcePod, registry, checkIntervalSec, output, checkRegistry)
					if err != nil {
						fmt.Println(fmt.Sprintf("error for %v: %v", registry.Name, err))
					} else {
						checkers[key] = checker
						fmt.Println(fmt.Sprintf("added registry check:%v (%v / user:%v)", registry.Name, registry.Url, registry.UserName))
					}
				}
			}

			for key, checker := range checkers {
				_, checkerIsAlreadyExists := registries[key]
				if !checkerIsAlreadyExists {
					// deleting outdated registry checks
					fmt.Println(fmt.Sprintf("deleted registry check: %v", key))

					checker.Destroy()
					delete(checkers, key)
				}
			}

		}
		time.Sleep(configRefreshInterval)
	}

}

func main() {
	var namespaceFlag *string
	var updateConfigSecFlag *int
	var checkIntervalSecFlag *int
	var podsPrefixFlag *string


	updateConfigSecFlag = flag.Int("updateConfigIntervalSec", 30, "interval in seconds between asking cluster for ping pods configuration")
	checkIntervalSecFlag = flag.Int("checkIntervalSec", 3, "interval between registry checks")
	namespaceFlag = flag.String("namespace", "monitoring", "current pod namespace where search registry secret records")
	podsPrefixFlag = flag.String("podsPrefix", "kubernetes-registry-check", "pods prefix")

	flag.Parse()

	fmt.Println(fmt.Sprintf("namespace = %s", *namespaceFlag))
	fmt.Println(fmt.Sprintf("updateConfigIntervalSec = %v", *updateConfigSecFlag))

	k8s, err := newK8s()
	if err != nil {
		panic(err)
	}
	fmt.Println(fmt.Sprintf("Started"))

	output := make(chan RegistryConnectionResultRecord)

	go func() {
		newRegistryPool(k8s, *namespaceFlag,*podsPrefixFlag, output, time.Duration(*updateConfigSecFlag)*time.Second, *checkIntervalSecFlag, checkRegistry)
	}()

	// write to output all records
	for {
		time.Sleep(10 * time.Millisecond)

		record, ok := <-output
		if !ok {
			break
		}

		fmt.Println(record.toString())
	}
}
