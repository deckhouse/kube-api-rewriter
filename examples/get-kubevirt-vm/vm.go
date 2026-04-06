package main

import (
	"fmt"

	"github.com/deckhouse/virtualization/api/client/kubeclient"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newVMCommand(root *rootOptions) *cobra.Command {
	o := &vmOptions{root: root}

	cmd := &cobra.Command{
		Use:   "vm [name]",
		Short: "Get virtual machines",
		Args:  cobra.MaximumNArgs(1),
		RunE:  o.Run,
	}

	cmd.Flags().BoolVarP(&o.watch, "watch", "w", false, "Watch virtual machines")

	return cmd
}

type vmOptions struct {
	root  *rootOptions
	watch bool
}

func (o *vmOptions) Run(cmd *cobra.Command, args []string) error {
	restConfig, namespace, err := o.root.GetRESTConfig()
	if err != nil {
		return err
	}

	vmClient, err := kubeclient.GetClientFromRESTConfig(restConfig)
	if err != nil {
		return fmt.Errorf("create virtualization client: %w", err)
	}

	virtualMachines := vmClient.VirtualMachines(namespace)
	vmName := ""
	if len(args) > 0 {
		vmName = args[0]
	}

	if o.watch {
		return watchVirtualMachines(cmd.Context(), virtualMachines, vmName, o.root.PrepareWatchOutput, o.root.PrintWatchEvent)
	}

	if vmName == "" {
		vmList, err := virtualMachines.List(cmd.Context(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("list virtualmachines in %s: %w", namespace, err)
		}
		return o.root.PrintObject(vmList)
	}

	vm, err := virtualMachines.Get(cmd.Context(), vmName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get virtualmachine %s/%s: %w", namespace, vmName, err)
	}

	return o.root.PrintObject(vm)
}
