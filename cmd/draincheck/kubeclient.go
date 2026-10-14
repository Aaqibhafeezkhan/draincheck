package main

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// newClientset builds a clientset from the standard kubeconfig loading rules.
// kubeconfig and kubeContext override the defaults when non-empty.
func newClientset(kubeconfig, kubeContext string) (kubernetes.Interface, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = kubeconfig
	overrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContext}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build client: %w", err)
	}
	return client, nil
}
