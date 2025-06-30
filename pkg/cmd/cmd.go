package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/andrewheberle/simplecommand"
	"github.com/bep/simplecobra"
)

func addToAgent(name string) error {
	cmd := exec.Command("ssh-add", name)

	return cmd.Run()
}

func removeFromAgent(name string) error {
	cmd := exec.Command("ssh-add", "-d", name)

	return cmd.Run()
}

func renameIdentity(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("problem renaming private key: %w", err)
	}
	if err := os.Rename(src+"-cert.pub", dst+"-cert.pub"); err != nil {
		return fmt.Errorf("problem renaming certificate: %w", err)
	}

	return nil
}

func identityFresh(name string, age time.Duration) bool {
	// stat file
	stat, err := os.Stat(name)
	if err != nil {
		return false
	}

	// if file is older than time.Now() - age then its not fresh
	if stat.ModTime().Before(time.Now().Add(-age)) {
		return false
	}

	// otherwise its fresh
	return true
}

type rootCommand struct {
	debug        bool
	forceRenewal bool
	name         string
	age          time.Duration

	logger *slog.Logger

	*simplecommand.Command
}

func (c *rootCommand) Init(cd *simplecobra.Commandeer) error {
	if err := c.Command.Init(cd); err != nil {
		return err
	}

	// command line flags
	cmd := cd.CobraCommand
	cmd.Flags().StringVar(&c.name, "name", "id_opkssh", "Name for opkssh identity key/certificate file(s)")
	cmd.Flags().DurationVar(&c.age, "maxage", 23*time.Hour, "Maximum age until renewal is required")
	cmd.Flags().BoolVar(&c.forceRenewal, "force", false, "Force renewal")
	cmd.Flags().BoolVar(&c.debug, "debug", false, "Enable debug logging")

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

	return nil
}

func (c *rootCommand) Run(ctx context.Context, cd *simplecobra.Commandeer, args []string) error {
	// find home dir
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// ssh dir
	sshDir := filepath.Join(home, ".ssh")

	// use ~/.ssh/id_opkssh for key
	opkKey := filepath.Join(sshDir, c.name)

	// check if its fresh
	if identityFresh(opkKey, c.age) {
		if c.forceRenewal {
			c.logger.Info("continuing as renewal forced even though not required")
		} else {
			// just (re-)add to agent
			c.logger.Info("no renewal required but (re-)adding to SSH agent", "key", opkKey, "certificate", opkKey+"-cert.pub")
			return addToAgent(opkKey)
		}
	}

	// create temp dir
	tmpDir, err := os.MkdirTemp(sshDir, "opkssh*")
	if err != nil {
		return fmt.Errorf("could not create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// do opkssh login
	c.logger.Info("starting opkssh login flow")
	newOpkKey := filepath.Join(tmpDir, c.name)
	opkssh := exec.Command("opkssh", "login", "-i", newOpkKey)
	if err := opkssh.Run(); err != nil {
		return fmt.Errorf("problem with opkssh login: %w", err)
	}

	// rename cert file
	if err := os.Rename(newOpkKey+".pub", newOpkKey+"-cert.pub"); err != nil {
		return fmt.Errorf("problem renaming certificate: %w", err)
	}

	// add to ssh-agent
	c.logger.Info("adding new identity to ssh-agent")
	if err := addToAgent(newOpkKey); err != nil {
		return fmt.Errorf("problem adding to agent: %w", err)
	}

	// remove old identity
	c.logger.Info("removing old identity from ssh-agent")
	if err := removeFromAgent(opkKey); err != nil {
		return fmt.Errorf("problem removing old identity: %w", err)
	}

	// move new files into place
	c.logger.Info("moving new identity into place")
	if err := renameIdentity(newOpkKey, opkKey); err != nil {
		return err
	}

	return nil
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
