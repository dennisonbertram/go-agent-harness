// native-gui-acceptance validates a rendered native proof bundle after a real
// macOS driver has collected it. It never accepts ToolWalk or a model reply as
// GUI evidence and never owns or kills an existing daemon/app process.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/internal/acceptance/inventory"
	"go-agent-harness/internal/acceptance/nativegui"
)

var (
	commandArgs           = os.Args[1:]
	stdout      io.Writer = os.Stdout
	stderr      io.Writer = os.Stderr
	exitFunc              = os.Exit
	runMain               = run
)

func main() {
	if err := runMain(commandArgs, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "native-gui-acceptance:", err)
		exitFunc(1)
	}
}

func run(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("native-gui-acceptance", flag.ContinueOnError)
	flags.SetOutput(errOut)
	url := flags.String("harness-url", "", "isolated harnessd base URL")
	manifestPath := flags.String("manifest", "", "native proof manifest JSON")
	artifactRoot := flags.String("artifact-root", "", "absolute artifact directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *url == "" || *manifestPath == "" || *artifactRoot == "" {
		return fmt.Errorf("-harness-url, -manifest, and -artifact-root are required")
	}
	compiled, err := liveInventory(*url)
	if err == nil {
		var m nativegui.Manifest
		data, readErr := os.ReadFile(*manifestPath)
		if readErr != nil {
			err = readErr
		} else if err = json.Unmarshal(data, &m); err == nil {
			err = validateLauncherBinding(strings.TrimRight(*url, "/"), m)
			if err == nil {
				err = nativegui.Validate(compiled, *artifactRoot, m)
			}
		}
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "native GUI evidence validated")
	return nil
}

// validateLauncherBinding prevents a driver from substituting a prior proof
// pack after the launcher has created this collection. The launcher exports
// these values only for its one child driver and the manifest must echo them.
func validateLauncherBinding(harnessURL string, manifest nativegui.Manifest) error {
	collection := manifest.Collection
	expected := map[string]string{
		"NATIVE_GUI_COLLECTION_LAUNCHER":        collection.Launcher,
		"NATIVE_GUI_COLLECTION_NONCE":           collection.Nonce,
		"NATIVE_GUI_COLLECTION_TEMP_ROOT":       collection.TempRoot,
		"NATIVE_GUI_COLLECTION_ARTIFACT_ROOT":   collection.ArtifactRoot,
		"NATIVE_GUI_COLLECTION_REPOSITORY_ROOT": collection.RepositoryRoot,
		"NATIVE_GUI_COLLECTION_DRIVER_PATH":     collection.DriverPath,
		"NATIVE_GUI_COLLECTION_DRIVER_DIGEST":   collection.DriverDigest,
	}
	for name, actual := range expected {
		if want := os.Getenv(name); want == "" || want != actual {
			return fmt.Errorf("manifest collection is not bound to launcher %s", name)
		}
	}
	if strings.TrimRight(collection.DaemonURL, "/") != harnessURL {
		return fmt.Errorf("manifest daemon URL does not match launcher URL")
	}
	return nil
}

func liveInventory(endpoint string) (inventory.Compiled, error) {
	response, err := (&http.Client{Timeout: 10 * time.Second}).Get(strings.TrimRight(endpoint, "/") + "/v1/tools")
	if err != nil {
		return inventory.Compiled{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return inventory.Compiled{}, fmt.Errorf("GET /v1/tools: %s", response.Status)
	}
	var p struct {
		Tools       []inventory.HTTPTool             `json:"tools"`
		Configured  *[]inventory.ConfiguredToolset   `json:"configured_unavailable_toolsets"`
		Unavailable *[]inventory.ResolverObservation `json:"unavailable"`
	}
	if err := json.NewDecoder(response.Body).Decode(&p); err != nil {
		return inventory.Compiled{}, err
	}
	if p.Configured == nil || p.Unavailable == nil {
		return inventory.Compiled{}, fmt.Errorf("/v1/tools omitted resolver evidence")
	}
	return inventory.Compile(inventory.InputFromHTTPBoundary(p.Tools, tui.NewCommandRegistry().All(), *p.Unavailable, *p.Configured))
}
