package cmd

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/andrewheberle/opkssh-renewer/pkg/opkssh"
	"github.com/andrewheberle/simplecommand"
	"github.com/bep/simplecobra"
)

type rootCommand struct {
	debug        bool
	forceRenewal bool
	name         string
	age          time.Duration

	logger  *slog.Logger
	renewer *opkssh.Renewer

	*simplecommand.Command
}

func (c *rootCommand) Init(cd *simplecobra.Commandeer) error {
	if err := c.Command.Init(cd); err != nil {
		return err
	}

	// command line flags
	cmd := cd.CobraCommand
	cmd.PersistentFlags().StringVar(&c.name, "name", "id_opkssh", "Name for opkssh identity key/certificate file(s)")
	cmd.PersistentFlags().DurationVar(&c.age, "maxage", 23*time.Hour, "Maximum age until renewal is required")
	cmd.PersistentFlags().BoolVar(&c.forceRenewal, "force", false, "Force renewal")
	cmd.PersistentFlags().BoolVar(&c.debug, "debug", false, "Enable debug logging")

	return nil
}

func (c *rootCommand) PreRun(this, runner *simplecobra.Commandeer) error {
	if err := c.Command.PreRun(this, runner); err != nil {
		return err
	}

	// set up logger
	logLevel := new(slog.LevelVar)
	c.logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	if c.debug {
		logLevel.Set(slog.LevelDebug)
	}

	renewer, err := opkssh.NewRenewer(c.name, c.age, c.forceRenewal, opkssh.WithLogger(c.logger))
	if err != nil {
		return err
	}
	c.renewer = renewer

	return nil
}

func (c *rootCommand) Run(ctx context.Context, cd *simplecobra.Commandeer, args []string) error {
	return c.renewer.Run()
}

func Execute(args []string) error {
	// Set up command
	root := &rootCommand{
		Command: simplecommand.New(
			"opkssh-renewal",
			"Handle renewal of OpkSSH keys as required",
		),
	}

	// Set up simplecobra
	x, err := simplecobra.New(root)
	if err != nil {
		return err
	}

	// run things
	if _, err := x.Execute(context.Background(), args); err != nil {
		return err
	}

	return nil
}
