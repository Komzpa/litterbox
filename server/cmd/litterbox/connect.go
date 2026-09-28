package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Komzpa/litterbox/server/internal/gmail"
)

// ConnectCredentialsEnv names the environment variable holding the path to
// the Google OAuth Desktop-client JSON. The file lives outside the
// repository (R17); a flag overrides the variable.
const ConnectCredentialsEnv = "LITTERBOX_GOOGLE_CLIENT_JSON"

// RunConnect implements the administrative `connect` subcommand: it runs
// the Gmail OAuth loopback flow and prints the refresh token and account
// email on stdout for the operator to store in the accounts table.
//
// main.go currently has no subcommand dispatch and is owned elsewhere; it
// wires this in with:
//
//	if len(os.Args) > 1 && os.Args[1] == "connect" {
//		os.Exit(RunConnect(os.Args[2:], os.Stdout))
//	}
//
// before flag.Parse().
func RunConnect(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(stdout)
	credentialsPath := fs.String("credentials", os.Getenv(ConnectCredentialsEnv),
		"path to the Google OAuth Desktop-client JSON (or "+ConnectCredentialsEnv+")")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *credentialsPath == "" {
		fmt.Fprintln(stdout, "connect: -credentials flag or "+ConnectCredentialsEnv+" is required")
		return 2
	}

	creds, err := gmail.LoadCredentials(*credentialsPath)
	if err != nil {
		fmt.Fprintln(stdout, "connect:", err)
		return 1
	}
	result, err := gmail.Connect(context.Background(), creds, &gmail.ConnectOptions{Out: stdout})
	if err != nil {
		fmt.Fprintln(stdout, "connect:", err)
		return 1
	}
	out, err := json.Marshal(map[string]string{
		"email":         result.Email,
		"refresh_token": result.RefreshToken,
	})
	if err != nil {
		fmt.Fprintln(stdout, "connect:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", out)
	return 0
}
