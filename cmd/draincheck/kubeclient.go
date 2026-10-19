package main

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// kubeconfigRules returns the standard kubeconfig loading rules, limited to
// the file at explicit when it is non-empty.
func kubeconfigRules(explicit string) *clientcmd.ClientConfigLoadingRules {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = explicit
	return rules
}

// kubeconfigPaths lists the kubeconfig files newClientset reads: the explicit
// path alone, or else the files named by KUBECONFIG, or else the default file
// in the home directory.
func kubeconfigPaths(explicit string) []string {
	return kubeconfigRules(explicit).GetLoadingPrecedence()
}

// newClientset builds a clientset from the standard kubeconfig loading rules.
// kubeconfig and kubeContext override the defaults when non-empty.
func newClientset(kubeconfig, kubeContext string) (kubernetes.Interface, error) {
	rules := kubeconfigRules(kubeconfig)
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
