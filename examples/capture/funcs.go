// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/gostafa/goinput"
)

func main() {
	reportResult(context.Background(), run(os.Args[exitFailure:]), os.Exit)
}

func reportResult(ctx context.Context, err error, exit func(int)) {
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		slog.New(slog.NewTextHandler(os.Stderr, nil)).
			ErrorContext(ctx, "capture failed", slog.Any(errorAttribute, err))
		exit(exitFailure)
	}
}

func run(args []string) error {
	options, err := parseOptions(args)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	defer stop()

	return errors.Join(runCycles(ctx, &options, systemCreator(ctx)))
}

func systemCreator(ctx context.Context) func() (*goinput.System, error) {
	return func() (*goinput.System, error) {
		return goinput.NewSystem(ctx)
	}
}

func parseOptions(args []string) (captureOptions, error) {
	flags := newCaptureFlags()

	err := flags.flags.Parse(args)
	if err != nil {
		return captureOptions{}, fmt.Errorf("parseOptions: %w", err)
	}

	options := optionsFromFlags(flags)

	err = validateOptions(&options)

	return options, errors.Join(err)
}

func newCaptureFlags() *captureFlags {
	flags := flag.NewFlagSet("capture", flag.ContinueOnError)

	return &captureFlags{
		flags:    flags,
		cycles:   flags.Int("cycles", exitFailure, "number of discovery/capture lifecycles"),
		managers: flags.Int("managers", exitFailure, "number of simultaneously open managers"),
		duration: flags.Duration(
			"duration",
			unlimitedTime,
			"optional duration of each device capture",
		),
	}
}

func optionsFromFlags(flags *captureFlags) captureOptions {
	return captureOptions{
		cycles:   *flags.cycles,
		managers: *flags.managers,
		duration: *flags.duration,
		args:     flags.flags.Args(),
		output:   os.Stdout,
		logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
}

func validateOptions(options *captureOptions) error {
	return errors.Join(
		validateCounts(options.cycles, options.managers),
		validateArguments(options.duration, options.args),
	)
}

func validateCounts(cycles, managers int) error {
	if cycles < exitFailure || managers < exitFailure {
		return errors.Join(usageError())
	}

	return nil
}

func validateArguments(duration time.Duration, args []string) error {
	if duration < unlimitedTime || len(args) > exitFailure {
		return errors.Join(usageError())
	}

	return nil
}

func usageError() error {
	return errUsage
}

func runCycles(
	ctx context.Context,
	options *captureOptions,
	createSystem func() (*goinput.System, error),
) error {
	system, err := createSystem()
	if err != nil {
		return fmt.Errorf("create input system: %w", err)
	}

	for range options.cycles {
		err = cycle(ctx, options, system)
		if err != nil {
			return fmt.Errorf("runCycles: %w", err)
		}
	}

	return nil
}

func cycle(ctx context.Context, options *captureOptions, system *goinput.System) error {
	managers, err := extraManagers(ctx, options, system)
	defer closeManagers(ctx, options.logger, managers)

	if err != nil {
		return fmt.Errorf(cycleErrorFormat, err)
	}

	manager, err := goinput.New(ctx, goinput.Options{BufferSize: 0}, system)
	if err != nil {
		return fmt.Errorf(cycleErrorFormat, err)
	}
	defer closeManager(ctx, options.logger, manager)

	return errors.Join(discoverAndCapture(ctx, manager, options), ctx.Err())
}

func extraManagers(
	ctx context.Context,
	options *captureOptions,
	system *goinput.System,
) ([]*goinput.Manager, error) {
	managers := make(
		[]*goinput.Manager,
		unlimitedTime,
		max(unlimitedTime, options.managers-exitFailure),
	)

	for index := exitFailure; index < options.managers; index++ {
		manager, err := openExtraManager(ctx, system, options)
		if err != nil {
			return managers, fmt.Errorf("extraManagers: %w", err)
		}

		managers = append(managers, manager)
	}

	return managers, nil
}

func closeManagers(ctx context.Context, logger *slog.Logger, managers []*goinput.Manager) {
	for index := range managers {
		closeManager(ctx, logger, managers[index])
	}
}

func closeManager(ctx context.Context, logger *slog.Logger, manager *goinput.Manager) {
	err := manager.Close()
	if err != nil {
		logger.ErrorContext(ctx, "close manager", slog.Any(errorAttribute, err))
	}
}

func discoverAndCapture(
	ctx context.Context,
	manager *goinput.Manager,
	options *captureOptions,
) error {
	infos, diagnostic := manager.Devices(ctx)
	if diagnostic != nil {
		options.logger.ErrorContext(ctx, "discovery", slog.Any(errorAttribute, diagnostic))
	}

	err := printDevices(options.output, infos)
	if err != nil {
		return fmt.Errorf("discoverAndCapture: %w", err)
	}

	if len(options.args) == unlimitedTime {
		return nil
	}

	return errors.Join(captureDevice(ctx, manager, options))
}

func printDevices(output io.Writer, infos []goinput.DeviceInfo) error {
	for index := range infos {
		err := resultError(
			fmt.Fprintf(output, "%s\t%s\n", infos[index].ID, infos[index].Name),
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
	defer closeDevice(ctx, options.logger, device)

	readctx, cancel := context.WithCancel(ctx)

	if options.duration > unlimitedTime {
		cancel()

		readctx, cancel = context.WithTimeout(ctx, options.duration)
	}

	defer cancel()

	return errors.Join(readEvents(readctx, device, options.output))
}

func closeDevice(ctx context.Context, logger *slog.Logger, device goinput.Device) {
	err := device.Close()
	if err != nil {
		logger.ErrorContext(ctx, "close device", slog.Any(errorAttribute, err))
	}
}

func readEvents(ctx context.Context, device goinput.Device, output io.Writer) error {
	controls := deviceControls(device)

	for {
		event, err := device.Read(ctx)
		if err != nil {
			return errors.Join(readError(err))
		}

		err = printEvent(output, &event, controls[event.ControlID])
		if err != nil {
			return errors.Join(err)
		}
	}
}

func deviceControls(device goinput.Device) map[goinput.ControlID]*goinput.Control {
	controls := make(map[goinput.ControlID]*goinput.Control)

	snapshot := device.Capabilities()

	for index := range snapshot.Controls {
		controls[snapshot.Controls[index].ID] = &snapshot.Controls[index]
	}

	return controls
}

func readError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}

	return err
}

func printEvent(output io.Writer, event *goinput.Event, control *goinput.Control) error {
	err := resultError(
		fmt.Fprintf(output, "%s\t%s\tusage=%s action=%d value=%g unit=%d time=%s\n",
			event.DeviceID, event.ControlID, controlUsage(control), event.Action, event.Value,
			controlUnit(control), event.Timestamp.Time.Format(eventTimeFormat)),
	)

	return errors.Join(err)
}

func controlUsage(control *goinput.Control) goinput.Usage {
	if control == nil {
		return goinput.UsageUnknown
	}

	return control.Usage
}

func controlUnit(control *goinput.Control) goinput.Unit {
	if control == nil {
		return goinput.UnitUnknown
	}

	return control.Unit
}

func openExtraManager(
	ctx context.Context,
	system *goinput.System,
	options *captureOptions,
) (*goinput.Manager, error) {
	manager, err := goinput.New(ctx, goinput.Options{BufferSize: unlimitedTime}, system)
	if err != nil {
		return nil, errors.Join(err)
	}

	err = resultError(manager.Devices(ctx))
	if err != nil {
		options.logger.ErrorContext(ctx, "additional manager", slog.Any(errorAttribute, err))
	}

	return manager, nil
}

// resultError retains the error when an operation's value is irrelevant.
func resultError[T any](_ T, err error) error { return errors.Join(err) }
