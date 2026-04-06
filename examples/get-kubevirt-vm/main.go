package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/deckhouse/kube-api-rewriter/pkg/proxy"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os/signal"
	"sigs.k8s.io/yaml"
	"syscall"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if err := newRootCommand().ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	o := newRootOptions()

	cmd := &cobra.Command{
		Use:           "get-kubevirt-vm",
		Short:         "Get kubevirt virtual machine",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.BindPersistentFlags(cmd)

	cmd.AddCommand(
		newVMCommand(o),
		newKVVMICommand(o),
	)

	return cmd
}

type rootOptions struct {
	loadingRules *clientcmd.ClientConfigLoadingRules
	overrides    *clientcmd.ConfigOverrides
	output       string
}

func newRootOptions() *rootOptions {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	return &rootOptions{
		loadingRules: loadingRules,
		overrides:    overrides,
	}
}

func (o *rootOptions) BindPersistentFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringVar(&o.loadingRules.ExplicitPath, "kubeconfig", "", "Path to the kubeconfig file to use for requests.")
	flags.StringVarP(&o.output, "output", "o", "", "Output format. One of: json|yaml. Defaults to table.")

	flagNames := clientcmd.RecommendedConfigOverrideFlags("")
	flagNames.ClusterOverrideFlags.APIServer.ShortName = "s"
	clientcmd.BindOverrideFlags(o.overrides, flags, flagNames)
}

func (o *rootOptions) ClientConfig() clientcmd.ClientConfig {
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(o.loadingRules, o.overrides, os.Stdin)
}

func (o *rootOptions) GetRESTConfig() (*rest.Config, string, error) {
	clientConfig := o.ClientConfig()

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("get rest config: %w", err)
	}

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("get namespace: %w", err)
	}

	rewriteRules := KubevirtRewriteRules
	rewriteRules.Init()

	proxy.WrapRESTConfig(restConfig, proxy.NewProxyRoundTripper("kubevirt-example", proxy.ToRenamed, rewriteRules))

	return restConfig, namespace, nil
}

func (o *rootOptions) PrintObject(obj interface{}) error {
	switch o.output {
	case "", "table":
		rows, err := tableRows(obj, "")
		if err != nil {
			return err
		}
		return printTable(os.Stdout, tableHeaders(false), rows)
	case "yaml":
		return printYAML(obj)
	case "json":
		return printJSON(obj)
	default:
		return fmt.Errorf("unsupported output format %q, expected one of: json, yaml", o.output)
	}
}

func (o *rootOptions) PrepareWatchOutput() error {
	if o.output == "" || o.output == "table" {
		return printTable(os.Stdout, tableHeaders(true), nil)
	}
	return nil
}

func (o *rootOptions) PrintWatchEvent(event watch.Event) error {
	switch o.output {
	case "", "table":
		rows, err := tableRows(event.Object, string(event.Type))
		if err != nil {
			return err
		}
		return printTableRows(os.Stdout, rows, true)
	case "yaml":
		return printYAML(map[string]interface{}{
			"type":   event.Type,
			"object": event.Object,
		})
	case "json":
		return printJSON(map[string]interface{}{
			"type":   event.Type,
			"object": event.Object,
		})
	default:
		return fmt.Errorf("unsupported output format %q, expected one of: json, yaml", o.output)
	}
}

func watchVirtualMachines(ctx context.Context, vmClient interface {
	Watch(context.Context, metav1.ListOptions) (watch.Interface, error)
}, vmName string, prepareOutput func() error, printEvent func(watch.Event) error) error {
	opts := metav1.ListOptions{}
	if vmName != "" {
		opts.FieldSelector = "metadata.name=" + vmName
	}

	w, err := vmClient.Watch(ctx, opts)
	if err != nil {
		return fmt.Errorf("watch virtualmachines: %w", err)
	}
	defer w.Stop()

	if prepareOutput != nil {
		if err := prepareOutput(); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-w.ResultChan():
			if !ok {
				return nil
			}
			if err := printEvent(event); err != nil {
				return err
			}
		}
	}
}

func printJSON(obj interface{}) error {
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	_, err = fmt.Fprintln(os.Stdout, string(out))
	return err
}

func printYAML(obj interface{}) error {
	out, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	_, err = fmt.Fprintln(os.Stdout, string(out))
	return err
}

type tableRow struct {
	EventType string
	Name      string
	Namespace string
	Phase     string
}

func tableHeaders(includeEventType bool) []string {
	if includeEventType {
		return []string{"TYPE", "NAME", "NAMESPACE", "PHASE"}
	}
	return []string{"NAME", "NAMESPACE", "PHASE"}
}

func printTable(out *os.File, headers []string, rows []tableRow) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}

	if err := writeTableRows(tw, rows, len(headers) == 4); err != nil {
		return err
	}

	return tw.Flush()
}

func printTableRows(out *os.File, rows []tableRow, includeEventType bool) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if err := writeTableRows(tw, rows, includeEventType); err != nil {
		return err
	}
	return tw.Flush()
}

func writeTableRows(tw *tabwriter.Writer, rows []tableRow, includeEventType bool) error {
	for _, row := range rows {
		if includeEventType {
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", row.EventType, row.Name, row.Namespace, row.Phase); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", row.Name, row.Namespace, row.Phase); err != nil {
			return err
		}
	}
	return nil
}

func tableRows(obj interface{}, eventType string) ([]tableRow, error) {
	value := reflect.ValueOf(obj)
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}

	if !value.IsValid() {
		return nil, nil
	}

	items := value.FieldByName("Items")
	if items.IsValid() && items.Kind() == reflect.Slice {
		rows := make([]tableRow, 0, items.Len())
		for i := 0; i < items.Len(); i++ {
			row, err := tableRowForValue(items.Index(i), eventType)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
		return rows, nil
	}

	row, err := tableRowForValue(value, eventType)
	if err != nil {
		return nil, err
	}
	return []tableRow{row}, nil
}

func tableRowForValue(value reflect.Value, eventType string) (tableRow, error) {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return tableRow{}, nil
		}
		value = value.Elem()
	}

	obj, err := valueAsMetaObject(value)
	if err != nil {
		return tableRow{}, err
	}

	return tableRow{
		EventType: eventType,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Phase:     phaseFromValue(value),
	}, nil
}

func valueAsMetaObject(value reflect.Value) (metav1.Object, error) {
	if !value.IsValid() {
		return nil, fmt.Errorf("invalid object")
	}
	if value.CanAddr() {
		if obj, ok := value.Addr().Interface().(metav1.Object); ok {
			return obj, nil
		}
	}
	if obj, ok := value.Interface().(metav1.Object); ok {
		return obj, nil
	}
	return nil, fmt.Errorf("object does not implement metav1.Object")
}

func phaseFromValue(value reflect.Value) string {
	status := value.FieldByName("Status")
	if !status.IsValid() {
		return ""
	}
	phase := status.FieldByName("Phase")
	if !phase.IsValid() {
		return ""
	}
	return fmt.Sprint(phase.Interface())
}
