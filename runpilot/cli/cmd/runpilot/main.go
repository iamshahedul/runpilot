package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"runpilot/cli/internal/client"
	"runpilot/shared/contracts"
	"runpilot/shared/models"
)

const version = "1.0.0"

func printUsage() {
	fmt.Printf(`RunPilot CLI v%s - Developer-First Node.js & Python Hosting

Usage:
  runpilot login                                Login to RunPilot control plane
  runpilot apps list                            List all applications
  runpilot apps create [options]                Create a new application
  runpilot deploy <app_id>                      Trigger deployment from Git
  runpilot restart <app_id>                     Restart application process
  runpilot status <app_id>                      Inspect application status
  runpilot logs <app_id> [--lines 50]           View application journal logs
  runpilot servers list                         List registered servers
  runpilot version                              Show version info

Create Options:
  --id        Application ID (slug)
  --name      Application display name
  --runtime   Runtime type (nodejs or python)
  --version   Runtime version (e.g. 20.x, 3.11)
  --repo      Git repository URL
  --branch    Git branch (default: main)
  --port      Internal port (default: 3000)
  --build     Build command
  --start     Start command
`, version)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	c, err := client.NewCLIClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing client: %v\n", err)
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "version":
		fmt.Printf("RunPilot CLI v%s (Zero PHP/Composer native runtime)\n", version)

	case "login":
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Control Plane Endpoint [http://127.0.0.1:8080]: ")
		endpoint, _ := reader.ReadString('\n')
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" {
			endpoint = "http://127.0.0.1:8080"
		}

		fmt.Print("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)

		fmt.Print("Password: ")
		pass, _ := reader.ReadString('\n')
		pass = strings.TrimSpace(pass)

		if err := c.Login(email, pass, endpoint); err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Successfully authenticated! Token saved to ~/.runpilot/credentials.json")

	case "apps":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		subCmd := os.Args[2]
		switch subCmd {
		case "list":
			apps, err := c.ListApps()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list apps: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%-18s %-20s %-10s %-8s %-10s %-25s\n", "APP ID", "NAME", "RUNTIME", "PORT", "STATUS", "CURRENT RELEASE")
			fmt.Println(strings.Repeat("-", 95))
			for _, a := range apps {
				fmt.Printf("%-18s %-20s %-10s %-8d %-10s %-25s\n",
					a.ID, a.Name, a.RuntimeType, a.Port, a.Status, a.CurrentRelease)
			}

		case "create":
			fs := flag.NewFlagSet("create", flag.ExitOnError)
			id := fs.String("id", "", "App ID")
			name := fs.String("name", "", "App Name")
			runtimeType := fs.String("runtime", "nodejs", "Runtime type (nodejs or python)")
			rtVersion := fs.String("version", "20.x", "Runtime version")
			repo := fs.String("repo", "", "Git repository")
			branch := fs.String("branch", "main", "Git branch")
			port := fs.Int("port", 3000, "Port")
			buildCmd := fs.String("build", "", "Build command")
			startCmd := fs.String("start", "", "Start command")
			_ = fs.Parse(os.Args[3:])

			if *name == "" || *repo == "" {
				fmt.Fprintln(os.Stderr, "Error: --name and --repo are required")
				os.Exit(1)
			}

			app := models.Application{
				ID:             *id,
				Name:           *name,
				RuntimeType:    contracts.RuntimeType(*runtimeType),
				RuntimeVersion: *rtVersion,
				GitRepo:        *repo,
				GitBranch:      *branch,
				Port:           *port,
				BuildCommand:   *buildCmd,
				StartCommand:   *startCmd,
			}

			created, err := c.CreateApp(app)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create application: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✓ Created application: %s (ID: %s)\n", created.Name, created.ID)
			fmt.Printf("Run 'runpilot deploy %s' to trigger first build.\n", created.ID)

		default:
			printUsage()
		}

	case "deploy":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: runpilot deploy <app_id>")
			os.Exit(1)
		}
		appID := os.Args[2]
		fmt.Printf("Initiating deployment for %s...\n", appID)
		dep, err := c.Deploy(appID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Deployment failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Deployment %s completed with status: %s\n", dep.ID, dep.State)

	case "restart":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: runpilot restart <app_id>")
			os.Exit(1)
		}
		appID := os.Args[2]
		stat, err := c.Restart(appID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restart %s: %v\n", appID, err)
			os.Exit(1)
		}
		fmt.Printf("✓ Application %s restarted. Current status: %s (PID: %d)\n", appID, stat.State, stat.PID)

	case "status":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: runpilot status <app_id>")
			os.Exit(1)
		}
		appID := os.Args[2]
		app, err := c.Status(appID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch status: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Application:     %s (%s)\n", app.Name, app.ID)
		fmt.Printf("Runtime:         %s (%s)\n", app.RuntimeType, app.RuntimeVersion)
		fmt.Printf("Repository:      %s (%s)\n", app.GitRepo, app.GitBranch)
		fmt.Printf("Assigned User:   %s (Unprivileged native process)\n", app.AssignedUnix)
		fmt.Printf("Port:            %d\n", app.Port)
		fmt.Printf("Status:          %s\n", app.Status)
		fmt.Printf("Current Release: %s\n", app.CurrentRelease)
		fmt.Printf("Resource Limits: CPU %s | RAM %s | Tasks %d\n",
			app.ResourceLimits.CPUQuota, app.ResourceLimits.MemoryMax, app.ResourceLimits.TasksMax)

	case "logs":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: runpilot logs <app_id> [--lines 50]")
			os.Exit(1)
		}
		appID := os.Args[2]
		lines := 50
		if len(os.Args) >= 5 && os.Args[3] == "--lines" {
			lines, _ = strconv.Atoi(os.Args[4])
		}

		logLines, err := c.Logs(appID, lines)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch logs: %v\n", err)
			os.Exit(1)
		}
		for _, l := range logLines {
			fmt.Println(l)
		}

	case "servers":
		servers, err := c.ListServers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list servers: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%-4s %-20s %-15s %-10s\n", "ID", "NAME", "IP", "STATUS")
		fmt.Println(strings.Repeat("-", 55))
		for _, s := range servers {
			fmt.Printf("%-4v %-20v %-15v %-10v\n", s["id"], s["name"], s["ip"], s["status"])
		}

	default:
		printUsage()
		os.Exit(1)
	}
}
