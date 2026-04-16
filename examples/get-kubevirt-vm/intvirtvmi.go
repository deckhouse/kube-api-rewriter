package main

import (
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/kubevirt"
)

func newKVVMICommand(root *rootOptions) *cobra.Command {
	o := &intvirtvmiOptions{root: root}

	cmd := &cobra.Command{
		Use:   "intvirtvmi [name]",
		Short: "Get internal virtualization VM instances",
		Args:  cobra.MaximumNArgs(1),
		RunE:  o.Run,
	}

	cmd.Flags().BoolVarP(&o.watch, "watch", "w", false, "Watch virtual machine instances")

	return cmd
}

type intvirtvmiOptions struct {
	root  *rootOptions
	watch bool
}

func (o *intvirtvmiOptions) Run(cmd *cobra.Command, args []string) error {
	restConfig, namespace, err := o.root.GetRESTConfig()
	if err != nil {
		return err
	}

	virtClient, err := kubevirt.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("create kubevirt client: %w", err)
	}

	vmiClient := virtClient.KubevirtV1().VirtualMachineInstances(namespace)
	vmiName := ""
	if len(args) > 0 {
		vmiName = args[0]
	}

	if o.watch {
		return watchVirtualMachines(cmd.Context(), vmiClient, vmiName, o.root.PrepareWatchOutput, o.root.PrintWatchEvent)
	}

	if vmiName == "" {
		vmiList, err := vmiClient.List(cmd.Context(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("list virtualmachineinstances in %s: %w", namespace, err)
		}
		return o.root.PrintObject(vmiList)
	}

	vmi, err := vmiClient.Get(cmd.Context(), vmiName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get virtualmachineinstance %s/%s: %w", namespace, vmiName, err)
	}

	return o.root.PrintObject(vmi)
}
