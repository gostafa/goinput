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

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cycles := flag.Int("cycles", 1, "number of discovery/capture lifecycles")
	managers := flag.Int("managers", 1, "number of simultaneously open managers")
	duration := flag.Duration("duration", 0, "optional duration of each device capture")
	flag.Parse()
	if *cycles < 1 || *managers < 1 || *duration < 0 || flag.NArg() > 1 {
		return fmt.Errorf("usage: capture [-cycles n] [-managers n] [-duration 2s] [device-id]")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	for i := 0; i < *cycles; i++ {
		if err := cycle(ctx, *managers, *duration, flag.Args()); err != nil {
			return err
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil
}

func cycle(ctx context.Context, count int, duration time.Duration, args []string) error {
	// Additional managers retain independent leases on the same native session.
	for i := 1; i < count; i++ {
		extra, err := goinput.New(goinput.Options{})
		if err != nil {
			return err
		}
		defer extra.Close()
		if _, err := extra.Devices(ctx); err != nil {
			log.Printf("additional manager: %v", err)
		}
	}
	manager, err := goinput.New(goinput.Options{})
	if err != nil {
		return err
	}
	defer manager.Close()
	infos, diagnostic := manager.Devices(ctx)
	if diagnostic != nil {
		log.Printf("discovery: %v", diagnostic)
	}
	for _, info := range infos {
		fmt.Printf("%s\t%s\n", info.ID, info.Name)
	}
	if len(args) == 0 {
		return nil
	}
	device, err := manager.Open(ctx, goinput.DeviceID(args[0]))
	if err != nil {
		return err
	}
	defer device.Close()
	readctx := ctx
	if duration > 0 {
		var cancel context.CancelFunc
		readctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}
	controls := make(map[goinput.ControlID]goinput.Control)
	for _, control := range device.Capabilities().Controls {
		controls[control.ID] = control
	}
	for {
		event, err := device.Read(readctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		control := controls[event.ControlID]
		fmt.Printf("%s\t%s\tusage=%s action=%d value=%g unit=%d time=%s\n",
			event.DeviceID, event.ControlID, control.Usage, event.Action, event.Value,
			control.Unit, event.Timestamp.Time.Format("15:04:05.000000"))
	}
}
