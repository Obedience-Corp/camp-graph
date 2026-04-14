package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printHelp(os.Stdout)
		return nil
	}

	switch args[0] {
	case "help", "--help", "-h":
		printHelp(os.Stdout)
		return nil
	case "current":
		return runCurrent()
	case "stable":
		fs := flag.NewFlagSet("stable", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		level := fs.String("level", "patch", "release increment: patch, minor, or major")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return runStable(*level)
	case "tag":
		fs := flag.NewFlagSet("tag", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		version := fs.String("version", "", "explicit release tag to create and push")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *version == "" {
			return fmt.Errorf("missing required --version")
		}
		return runTag(*version)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printHelp(out *os.File) {
	fmt.Fprintln(out, "camp-graph release commands")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Primary:")
	fmt.Fprintln(out, "  just release stable [patch|minor|major]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Support:")
	fmt.Fprintln(out, "  just release current")
	fmt.Fprintln(out, "  just release check")
	fmt.Fprintln(out, "  just release snapshot")
	fmt.Fprintln(out, "  just release tag v0.1.0")
}
