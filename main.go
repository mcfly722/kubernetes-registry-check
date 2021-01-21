package main

import (
	"fmt"
	"time"
	"flag"
	"encoding/base64"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

/*
type DockerAuths struct {
	

}

type DockerJson struct {
	Auths DockerAuths

}
*/

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

func getRegistries(k8s *k8s, namespace string) ([]Registry, error) {

	registries := []Registry{}

	secrets, err := k8s.clientset.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	
	for _,secret :=range secrets.Items {
		for key := range secret.Data {
			if key == ".dockerconfigjson" {
				fmt.Println(fmt.Sprintf("[%v]", secret.ObjectMeta.Name))
			
				data, err := base64.StdEncoding.DecodeString(string(secret.Data[key]))
				if err == nil {

					registry := Registry {
						Name    : secret.ObjectMeta.Name,
						Url     : string(data),
						UserName: "one",
						Password: "two",
					}
		
					registries = append(registries, registry)
				} 
			}
		}
	}
	

	return registries,nil
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
	
	registries,err := getRegistries(k8s, *namespaceFlag)
	if err != nil {
		panic(err)
	}
	
	for i,_ := range registries {
		fmt.Println(fmt.Sprintf("registry:%v url:%v user:%v password:%v",registries[i].Name,registries[i].Url,registries[i].UserName,registries[i].Password))
	}
	
	time.Sleep(8 * time.Second) 
}