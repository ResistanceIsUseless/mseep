package main

import (
	"fmt"

	"mseep/internal/app"
)

func runTUI() error {
	return fmt.Errorf("TUI coming soon. Use CLI: enable/disable/toggle/status")
}

func cmdEnableDisableToggle(mode, q, client string, yes bool) error {
	a, err := app.LoadApp()
	if err != nil { return err }
	d, err := a.Toggle(mode, q, client, yes)
	if err != nil { return err }
	if d != "" { fmt.Println(d) }
	return nil
}

func runStatus(client string, json bool) error {
	a, err := app.LoadApp()
	if err != nil {
		return err
	}
	output, err := a.Status(client, json)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}
func runHealth(client, server string, fix bool) error {
	a, err := app.LoadApp()
	if err != nil {
		return err
	}
	output, err := a.Health(client, server, fix, false)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}
func runApply(client, profile string) error {
	a, err := app.LoadApp()
	if err != nil {
		return err
	}
	return a.Apply(client, profile, false)
}
