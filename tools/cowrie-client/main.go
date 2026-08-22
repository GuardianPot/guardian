package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

func main() {
	host := flag.String("host", "127.0.0.1", "SSH server host")
	port := flag.Int("port", 2222, "SSH server port")
	user := flag.String("user", "root", "SSH username")
	password := flag.String("password", "", "SSH password")
	command := flag.String("command", "", "remote command")
	interactive := flag.Bool("interactive", true, "run the command through an interactive shell")
	timeout := flag.Duration("timeout", 10*time.Second, "connection timeout")
	flag.Parse()

	if *password == "" || *command == "" {
		fmt.Fprintln(os.Stderr, "password and command are required")
		os.Exit(2)
	}

	config := &ssh.ClientConfig{
		User:            *user,
		Auth:            []ssh.AuthMethod{ssh.Password(*password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         *timeout,
	}

	address := net.JoinHostPort(*host, fmt.Sprintf("%d", *port))
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH dial failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH session failed: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	if *interactive {
		var output bytes.Buffer
		session.Stdout = &output
		session.Stderr = &output
		session.Stdin = bytes.NewBufferString(*command + "\nexit\n")
		if err := session.RequestPty("xterm", 120, 40, ssh.TerminalModes{}); err != nil {
			fmt.Fprintf(os.Stderr, "SSH PTY request failed: %v\n", err)
			os.Exit(1)
		}
		if err := session.Shell(); err != nil {
			fmt.Fprintf(os.Stderr, "SSH shell failed: %v\n", err)
			os.Exit(1)
		}
		err = session.Wait()
		_, _ = io.Copy(os.Stdout, &output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SSH shell failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	output, err := session.CombinedOutput(*command)
	fmt.Print(string(output))
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH command failed: %v\n", err)
		os.Exit(1)
	}
}
