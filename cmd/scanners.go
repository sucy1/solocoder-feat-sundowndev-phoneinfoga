package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/sundowndev/phoneinfoga/v2/lib/filter"
	"github.com/sundowndev/phoneinfoga/v2/lib/remote"
)

type ScannersCmdOptions struct {
	Plugin    []string
	PluginDir string
}

func init() {
	opts := &ScannersCmdOptions{}
	scannersCmd := NewScannersCmd(opts)

	fl := scannersCmd.Flags()
	fl.StringSliceVar(&opts.Plugin, "plugin", []string{}, "Plugin file to load")
	fl.StringVar(&opts.PluginDir, "plugin-dir", "", "Directory to load .so scanner plugins from")

	rootCmd.AddCommand(scannersCmd)
}

func NewScannersCmd(opts *ScannersCmdOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "scanners",
		Example: "phoneinfoga scanners",
		Short:   "Display list of loaded scanners",
		Run: func(cmd *cobra.Command, args []string) {
			for _, p := range opts.Plugin {
				err := remote.OpenPlugin(p)
				if err != nil {
					exitWithError(err)
				}
			}

			if opts.PluginDir != "" {
				errs := remote.LoadPluginDir(opts.PluginDir)
				for _, e := range errs {
					fmt.Printf("Warning: %v\n", e)
				}
			}

			remoteLibrary := remote.NewLibrary(filter.NewEngine())
			remote.InitScanners(remoteLibrary)

			for i, s := range remoteLibrary.GetAllScanners() {
				fmt.Printf("%s\n%s\n", s.Name(), s.Description())
				if i < len(remoteLibrary.GetAllScanners()) {
					fmt.Printf("\n")
				}
			}
		},
	}
	return cmd
}
