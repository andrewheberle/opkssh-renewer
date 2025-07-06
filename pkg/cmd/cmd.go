package cmd

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/andrewheberle/opkssh-renewer/pkg/sshagent"
	"github.com/andrewheberle/simplecommand"
	"github.com/bep/simplecobra"
	"github.com/openpubkey/opkssh/commands"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func loadKey(name string) (*ecdsa.PrivateKey, error) {
	pemBytes, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file %q: %w", name, err)
	}

	privateKey, err := ssh.ParseRawPrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("could not parse private key file: %w", err)
	}

	ecdsaKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key from %q is not an ECDSA key; its type is %T", name, privateKey)
	}

	return ecdsaKey, nil
}

func loadpubkey(name string) (ssh.PublicKey, string, error) {
	// Read the content of the file
	keyBytes, err := os.ReadFile(name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read SSH key/certificate file %q: %w", name, err)
	}

	// Parse the certificate.
	parsedKey, comment, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse SSH public key/certificate from %q: %w", name, err)
	}

	return parsedKey, comment, nil
}

func loadcert(name string) (*ssh.Certificate, string, error) {
	parsedKey, comment, err := loadpubkey(name)
	if err != nil {
		return nil, "", err
	}

	// Type assert to *ssh.Certificate. If it's not a certificate, this will fail.
	cert, ok := parsedKey.(*ssh.Certificate)
	if !ok {
		return nil, "", fmt.Errorf("the provided key in %q is not an SSH certificate, it is a %T", name, parsedKey)
	}

	return cert, comment, nil
}

func addToAgent(name string) error {
	key, err := loadKey(name)
	if err != nil {
		return fmt.Errorf("could not load private key: %w", err)
	}

	cert, comment, err := loadcert(name + "-cert.pub")
	if err != nil {
		return fmt.Errorf("could not load certificate: %w", err)
	}

	return addKeyCertToAgent(key, cert, comment)
}

func addKeyCertToAgent(key *ecdsa.PrivateKey, cert *ssh.Certificate, comment string) error {
	agentClient, err := sshagent.NewAgent()
	if err != nil {
		return fmt.Errorf("could not connect to agent: %w", err)
	}

	return agentClient.Add(agent.AddedKey{
		PrivateKey:  key,
		Certificate: cert,
		Comment:     comment,
	})
}

func removeFromAgent(name string) error {
	pub, _, err := loadpubkey(name)
	if err != nil {
		return err
	}

	agentClient, err := sshagent.NewAgent()
	if err != nil {
		return fmt.Errorf("could not connect to agent: %w", err)
	}

	return agentClient.Remove(pub)
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

func identityAge(name string) time.Duration {
	// stat file
	stat, err := os.Stat(name)
	if err != nil {
		return -1
	}

	return time.Since(stat.ModTime())
}

func mkdirTemp(dir, pattern string) (string, error) {
	// ensure parent dir exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	return os.MkdirTemp(dir, pattern)
}

func opksshLogin(keypath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// set up opkssh login command
	login := commands.NewLogin(false, "", false, "", false, false, false, "", keypath, "")

	// run it
	return login.Run(ctx)
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
	if age := identityAge(opkKey); age >= 0 {
		// if identityAge returns >= the identity file modification time could be found, so check if renewal is required/forced
		if age < c.age {
			if c.forceRenewal {
				c.logger.Info("continuing as renewal forced even though not required", "age", age)
			} else {
				// just (re-)add to agent
				c.logger.Info("no renewal required but (re-)adding to SSH agent", "key", opkKey, "certificate", opkKey+"-cert.pub", "age", age)
				return addToAgent(opkKey)
			}
		}
	}

	// create temp dir
	tmpDir, err := mkdirTemp(sshDir, "opkssh*")
	if err != nil {
		return fmt.Errorf("could not create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// do opkssh login
	c.logger.Info("starting opkssh login flow")
	newOpkKey := filepath.Join(tmpDir, c.name)
	if err := opksshLogin(newOpkKey); err != nil {
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
	if err := removeFromAgent(opkKey + "-cert.pub"); err != nil {
		// this is not fatal
		c.logger.Warn("problem removing old identity", "error", err)
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
