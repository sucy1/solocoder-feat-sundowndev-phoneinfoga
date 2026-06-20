package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/sundowndev/phoneinfoga/v2/lib/filter"
	"github.com/sundowndev/phoneinfoga/v2/lib/number"
	"github.com/sundowndev/phoneinfoga/v2/lib/output"
	"github.com/sundowndev/phoneinfoga/v2/lib/remote"
)

type ScanCmdOptions struct {
	Number           string
	DisabledScanners []string
	PluginPaths      []string
	PluginDir        string
	EnvFiles         []string
	OutputPath       string
	SkipValidation   bool
	Delay            string
	NoDedup          bool
}

func init() {
	opts := &ScanCmdOptions{}
	cmd := NewScanCmd(opts)
	rootCmd.AddCommand(cmd)

	cmd.PersistentFlags().StringVarP(&opts.Number, "number", "n", "", "The phone number to scan (E164 or international format)")
	cmd.PersistentFlags().StringArrayVarP(&opts.DisabledScanners, "disable", "D", []string{}, "Scanner to skip for this scan")
	cmd.PersistentFlags().StringArrayVar(&opts.PluginPaths, "plugin", []string{}, "Extra scanner plugin to use for the scan")
	cmd.PersistentFlags().StringVar(&opts.PluginDir, "plugin-dir", "", "Directory to load .so scanner plugins from")
	cmd.PersistentFlags().StringSliceVar(&opts.EnvFiles, "env-file", []string{}, "Env files to parse environment variables from (looks for .env by default)")
	cmd.PersistentFlags().StringVarP(&opts.OutputPath, "output", "o", "", "Output file path (supports .json, .csv, .html extensions)")
	cmd.PersistentFlags().BoolVar(&opts.SkipValidation, "skip-validation", false, "Skip E.164 phone number format validation")
	cmd.PersistentFlags().StringVar(&opts.Delay, "delay", "", "Delay between scanner requests (e.g. 2s for fixed, 1s-3s for random range). Only affects remote API scanners")
	cmd.PersistentFlags().BoolVar(&opts.NoDedup, "no-dedup", false, "Disable result deduplication")
}

func NewScanCmd(opts *ScanCmdOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan a phone number",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			err := godotenv.Load(opts.EnvFiles...)
			if err != nil {
				logrus.WithField("error", err).Debug("Error loading .env file")
			}

			runScan(opts)
		},
	}
}

func parseDelay(delayStr string) (remote.DelayConfig, error) {
	if delayStr == "" {
		return remote.DelayConfig{}, nil
	}

	if strings.Contains(delayStr, "-") {
		parts := strings.SplitN(delayStr, "-", 2)
		if len(parts) != 2 {
			return remote.DelayConfig{}, fmt.Errorf("invalid delay format: %q (expected '1s-3s' for random range or '2s' for fixed)", delayStr)
		}
		minDelay, err := time.ParseDuration(parts[0])
		if err != nil {
			return remote.DelayConfig{}, fmt.Errorf("invalid delay min value %q: %v", parts[0], err)
		}
		maxDelay, err := time.ParseDuration(parts[1])
		if err != nil {
			return remote.DelayConfig{}, fmt.Errorf("invalid delay max value %q: %v", parts[1], err)
		}
		if minDelay >= maxDelay {
			return remote.DelayConfig{}, fmt.Errorf("delay min value must be less than max value")
		}
		return remote.DelayConfig{
			IsRandom: true,
			MinDelay: minDelay,
			MaxDelay: maxDelay,
		}, nil
	}

	d, err := time.ParseDuration(delayStr)
	if err != nil {
		return remote.DelayConfig{}, fmt.Errorf("invalid delay format %q: %v", delayStr, err)
	}
	return remote.DelayConfig{
		Fixed: d,
	}, nil
}

func runScan(opts *ScanCmdOptions) {
	fmt.Fprintf(color.Output, color.WhiteString("Running scan for phone number %s...\n\n"), opts.Number)

	if !opts.SkipValidation {
		if err := number.ValidateE164(opts.Number); err != nil {
			exitWithError(err)
		}
	} else {
		if valid := number.IsValid(opts.Number); !valid {
			logrus.WithFields(map[string]interface{}{
				"input": opts.Number,
				"valid": valid,
			}).Debug("Input phone number is invalid")
			exitWithError(errors.New("given phone number is not valid"))
		}
	}

	num, err := number.NewNumber(opts.Number)
	if err != nil {
		exitWithError(err)
	}

	for _, p := range opts.PluginPaths {
		err := remote.OpenPlugin(p)
		if err != nil {
			exitWithError(err)
		}
	}

	if opts.PluginDir != "" {
		errs := remote.LoadPluginDir(opts.PluginDir)
		for _, e := range errs {
			logrus.WithError(e).Warn("Plugin load error")
		}
	}

	delayConfig, err := parseDelay(opts.Delay)
	if err != nil {
		exitWithError(err)
	}

	f := filter.NewEngine()
	f.AddRule(opts.DisabledScanners...)

	remoteLibrary := remote.NewLibrary(f)
	remoteLibrary.SetDelay(delayConfig)
	remote.InitScanners(remoteLibrary)

	result, errs := remoteLibrary.Scan(num, remote.ScannerOptions{})

	if !opts.NoDedup {
		dedupResult := remote.DeduplicateResults(result, errs)
		result = dedupResult.Result
		errs = dedupResult.Errors
		if dedupResult.BeforeCount != dedupResult.AfterCount {
			fmt.Fprintf(color.Output, color.CyanString("Deduplication: %d fields before, %d fields after (%d duplicates removed)\n\n"), dedupResult.BeforeCount, dedupResult.AfterCount, dedupResult.BeforeCount-dedupResult.AfterCount)
		}
	}

	var outputWriter output.Output
	var outputKey output.OutputKey

	if opts.OutputPath != "" {
		outputKey, err = output.OutputKeyFromPath(opts.OutputPath)
		if err != nil {
			exitWithError(err)
		}

		f, fileErr := os.Create(opts.OutputPath)
		if fileErr != nil {
			exitWithError(fmt.Errorf("failed to create output file: %v", fileErr))
		}
		defer f.Close()

		outputWriter = output.GetOutput(outputKey, f)
	} else {
		outputWriter = output.GetOutput(output.Console, color.Output)
	}

	if err = outputWriter.Write(result, errs); err != nil {
		exitWithError(err)
	}

	if opts.OutputPath != "" {
		fmt.Fprintf(color.Output, color.GreenString("Results saved to %s\n"), opts.OutputPath)
	}
}
