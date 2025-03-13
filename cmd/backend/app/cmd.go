package app

import (
	"k8s.io/klog/v2"
)

// Execute executes the commands.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}
