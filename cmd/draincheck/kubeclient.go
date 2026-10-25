package main

import (
	"fmt"
	"log/slog"
	"net/url"

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
// kubeconfig and kubeContext override the defaults when non-empty. Each step
// writes a DEBUG line: the server by host only, never credentials.
func newClientset(kubeconfig, kubeContext string, logger *slog.Logger) (kubernetes.Interface, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	rules := kubeconfigRules(kubeconfig)
	overrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContext}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		logger.Debug("load kubeconfig failed", "error", err)
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	logger.Debug("load kubeconfig", "server", serverHost(cfg.Host))
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Debug("build client failed", "error", err)
		return nil, fmt.Errorf("build client: %w", err)
	}
	logger.Debug("build client")
	return client, nil
}

// serverHost returns the host and port of a server URL, without the scheme,
// user info or path.
func serverHost(server string) string {
	u, err := url.Parse(server)
	if err != nil {
		return ""
	}
	return u.Host
}
