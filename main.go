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

type DockerAuth struct {
	Auth     string
	UserName string
	Password string
}

type DockerAuths struct {
	Auths map[string]DockerAuth
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

func getRegistries(k8s *k8s, namespace string) (map[string]Registry, error) {

	registries := map[string]Registry{}

	secrets, err := k8s.clientset.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	
	for _,secret :=range secrets.Items {
	
		b64data, ok := secret.Data[".dockerconfigjson"]
		if ok {
			var auths DockerAuths 
			
			err := json.Unmarshal(b64data, &auths)
			if err != nil {
				fmt.Println("decoding %v error:", secret.ObjectMeta.Name, err)
			} else {
			
				for key := range auths.Auths{
					_, keyIsAlreadyExists :=registries[key]
					
					if !keyIsAlreadyExists {
						registries[key] = Registry {
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


func main() {
	var namespaceFlag *string

	namespaceFlag = flag.String("namespace", "monitoring", "current pod namespace where search registry secret records")

	flag.Parse()

	fmt.Println(fmt.Sprintf("namespace = %s", *namespaceFlag))


	k8s, err := newK8s()
	if err != nil {
		panic(err)
	}
	fmt.Println(fmt.Sprintf("Started"))
	
	registries, err := getRegistries(k8s, *namespaceFlag)
	if err != nil {
		panic(err)
	}
	
	for _,r := range registries {
		fmt.Println(fmt.Sprintf("registry:%v url:%v user:%v",r.Name,r.Url,r.UserName))
	}
	
	time.Sleep(8 * time.Second) 
}