package main

import (
	"fmt"
	"time"
	"flag"
	"encoding/json"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type RegistryConnectionResultRecord struct {
	Url         string
	Message     string
	Success     bool
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

type k8s struct {
	clientset kubernetes.Interface
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
	
	for _,secret :=range secrets.Items {
	
		b64data, ok := secret.Data[".dockerconfigjson"]
		if ok {
			var auths RegistryAuths 
			
			err := json.Unmarshal(b64data, &auths)
			if err != nil {
				fmt.Println("decoding %v error:", secret.ObjectMeta.Name, err)
			} else {
			
				for key := range auths.Auths{
					_, keyIsAlreadyExists :=registries[key]
					
					if !keyIsAlreadyExists {
						registries[key] = &Registry {
							Name    : secret.ObjectMeta.Name,
							Url     : key,
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

func newRegistryPool(k8s *k8s, namespace string, output chan RegistryConnectionResultRecord,configRefreshInterval time.Duration, pingIntervalSec int) {

	registries := map[string]*Registry{}
	
	for {
		obtained_registries, err := getRegistries(k8s, namespace)
		if err != nil {
			fmt.Println(fmt.Sprintf("error: %v", err))
		} else {
			
			for key,registry := range obtained_registries {
				_, registryIsAlreadyExists :=registries[key]
				if !registryIsAlreadyExists {
					// add new registry check
					registries[key] = registry
					
					fmt.Println(fmt.Sprintf("adding registry check:%v (%v / user:%v)",registry.Name,registry.Url,registry.UserName))
				}
			}

			for key,registry := range registries {
				_, registryIsAlreadyExists :=obtained_registries[key]
				if !registryIsAlreadyExists {
					// deleting outdated registry checks
					fmt.Println(fmt.Sprintf("deleting registry check: (%v / user:%v)",registry.Name,registry.Url,registry.UserName))
					
					delete(registries, key)
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

	updateConfigSecFlag = flag.Int("updateConfigIntervalSec", 30, "interval in seconds between asking cluster for ping pods configuration")
	checkIntervalSecFlag = flag.Int("checkIntervalSec", 3, "interval between registry checks")
	namespaceFlag = flag.String("namespace", "monitoring", "current pod namespace where search registry secret records")

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
		newRegistryPool(k8s, *namespaceFlag, output, time.Duration(*updateConfigSecFlag)*time.Second, *checkIntervalSecFlag)
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