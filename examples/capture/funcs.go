// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gostafa/goinput"
)

type (
	captureOptions struct {
		args     []string
		cycles   int
		managers int
		duration time.Duration
	}
)

const (
	exitFailure   = 1
	unlimitedTime = 0

	eventTimeFormat = "15:04:05.000000"
	exitSuccess     = 0
)

func main() {
	err := run()
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		log.Print(err)
		os.Exit(exitFailure)
	}
}

func run() error {
	options, err := parseOptions()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	defer stop()

	return errors.Join(runCycles(ctx, &options))
}

func parseOptions() (captureOptions, error) {
	flags := flag.NewFlagSet("capture", flag.ContinueOnError)
	cycles := flags.Int("cycles", exitFailure, "number of discovery/capture lifecycles")
	managers := flags.Int("managers", exitFailure, "number of simultaneously open managers")
	duration := flags.Duration(
		"duration",
		unlimitedTime,
		"optional duration of each device capture",
	)

	err := flags.Parse(os.Args[exitFailure:])
	if err != nil {
		return captureOptions{}, fmt.Errorf("parseOptions: %w", err)
	}

	options, err := parsedOptions(flags, *cycles, *managers, *duration)

	return options, errors.Join(err)
}

func (options *captureOptions) validate() error {
	if options.cycles < exitFailure || options.managers < exitFailure {
		return errors.Join(usageError())
	}

	if options.duration < unlimitedTime || len(options.args) > exitFailure {
		return errors.Join(usageError())
	}

	return nil
}

func usageError() error {
	return errors.New("usage: capture [-cycles n] [-managers n] [-duration 2s] [device-id]")
}

func runCycles(ctx context.Context, options *captureOptions) error {
	system, err := goinput.NewSystem()
	if err != nil {
		return fmt.Errorf("create input system: %w", err)
	}

	for range options.cycles {
		err := cycle(ctx, options, system)
		if err != nil {
			return fmt.Errorf("runCycles: %w", err)
		}

		if ctx.Err() != nil {
			break
		}
	}

	return nil
}

func cycle(ctx context.Context, options *captureOptions, system *goinput.System) error {
	managers, err := extraManagers(ctx, options.managers, system)
	defer closeManagers(managers)

	if err != nil {
		return fmt.Errorf("cycle: %w", err)
	}

	manager, err := goinput.New(goinput.Options{BufferSize: 0}, system)
	if err != nil {
		return fmt.Errorf("cycle: %w", err)
	}
	defer closeManager(manager)

	return errors.Join(discoverAndCapture(ctx, manager, options))
}

func extraManagers(
	ctx context.Context,
	count int,
	system *goinput.System,
) ([]*goinput.Manager, error) {
	var managers []*goinput.Manager

	for index := exitFailure; index < count; index++ {
		manager, err := goinput.New(goinput.Options{BufferSize: 0}, system)
		if err != nil {
			return managers, fmt.Errorf("extraManagers: %w", err)
		}

		managers = append(managers, manager)
		if err := resultError(manager.Devices(ctx)); err != nil {
			log.Printf("additional manager: %v", err)
		}
	}

	return managers, nil
}

func closeManagers(managers []*goinput.Manager) {
	for index := range managers {
		closeManager(managers[index])
	}
}

func closeManager(manager *goinput.Manager) {
	err := manager.Close()
	if err != nil {
		log.Printf("close manager: %v", err)
	}
}

func discoverAndCapture(
	ctx context.Context,
	manager *goinput.Manager,
	options *captureOptions,
) error {
	infos, diagnostic := manager.Devices(ctx)
	if diagnostic != nil {
		log.Printf("discovery: %v", diagnostic)
	}

	err := printDevices(infos)
	if err != nil {
		return fmt.Errorf("discoverAndCapture: %w", err)
	}

	if len(options.args) == unlimitedTime {
		return nil
	}

	return errors.Join(captureDevice(ctx, manager, options))
}

func printDevices(infos []goinput.DeviceInfo) error {
	for index := range infos {
		err := resultError(
			fmt.Printf("%s\t%s\n", infos[index].ID, infos[index].Name),
		)
		if err != nil {
			return errors.Join(err)
		}
	}

	return nil
}

func captureDevice(ctx context.Context, manager *goinput.Manager, options *captureOptions) error {
	device, err := manager.Open(ctx, goinput.DeviceID(options.args[unlimitedTime]))
	if err != nil {
		return fmt.Errorf("captureDevice: %w", err)
	}
	defer closeDevice(device)

	readctx, cancel := captureContext(ctx, options.duration)

	defer cancel()

	return errors.Join(readEvents(readctx, device))
}

func closeDevice(device goinput.Device) {
	err := device.Close()
	if err != nil {
		log.Printf("close device: %v", err)
	}
}

func captureContext(
	ctx context.Context,
	duration time.Duration,
) (context.Context, context.CancelFunc) {
	if duration > unlimitedTime {
		return context.WithTimeout(ctx, duration)
	}

	return ctx, func() {}
}

func readEvents(ctx context.Context, device goinput.Device) error {
	controls := deviceControls(device)

	for {
		event, err := device.Read(ctx)
		if err != nil {
			return errors.Join(readError(err))
		}

		if err := printEvent(&event, controls[event.ControlID]); err != nil {
			return errors.Join(err)
		}
	}
}

func deviceControls(device goinput.Device) map[goinput.ControlID]*goinput.Control {
	controls := make(map[goinput.ControlID]*goinput.Control)

	for index := range device.Capabilities().Controls {
		controls[device.Capabilities().Controls[index].ID] = &device.Capabilities().Controls[index]
	}

	return controls
}

func readError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}

	return err
}

func printEvent(event *goinput.Event, control *goinput.Control) error {
	var err error

	err = resultError(fmt.Printf("%s\t%s\tusage=%s action=%d value=%g unit=%d time=%s\n",
		event.DeviceID, event.ControlID, controlUsage(control), event.Action, event.Value,
		controlUnit(control), event.Timestamp.Time.Format(eventTimeFormat)))

	return errors.Join(err)
}

func controlUsage(control *goinput.Control) goinput.Usage {
	if control == nil {
		return exitSuccess
	}

	return control.Usage
}

func controlUnit(control *goinput.Control) goinput.Unit {
	if control == nil {
		return exitSuccess
	}

	return control.Unit
}

func parsedOptions(
	flags *flag.FlagSet,
	cycles, managers int,
	duration time.Duration,
) (captureOptions, error) {
	options := captureOptions{
		cycles:   cycles,
		managers: managers,
		duration: duration,
		args:     flags.Args(),
	}

	return options, errors.Join(options.validate())
}
