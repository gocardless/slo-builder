package main

import (
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"reflect"
	"runtime"

	yaml "gopkg.in/yaml.v2"

	"github.com/alecthomas/kingpin"
	kitlog "github.com/go-kit/kit/log"

	"github.com/gocardless/slo-builder/pkg/templates"
)

var logger kitlog.Logger

var (
	app = kingpin.New("slo-builder", "Build a Prometheus SLO pipelines from SLO templates").Version(versionStanza())

	listTemplates = app.Command("list-templates", "Lists available SLO templates")

	build               = app.Command("build", "Builds the Prometheus RuleGroup from given SLO definitions")
	buildName           = build.Flag("name", "Name of the generated Prometheus RuleGroup").Default("slo-builder").String()
	buildSloDefinitions = build.Arg("slo-definitions", "Files containing list of SLO template instances").Strings()
)

func main() {
	logger = kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stderr))
	stdlog.SetOutput(kitlog.NewStdlibAdapter(logger))
	logger = kitlog.With(logger, "ts", kitlog.DefaultTimestampUTC, "caller", kitlog.DefaultCaller)

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case listTemplates.FullCommand():
		for templateName, _ := range templates.Templates {
			fmt.Println(templateName)
		}

	case build.FullCommand():
		slos, err := loadDefinitions(*buildSloDefinitions)
		if err != nil {
			logger.Log("error", err, "msg", "failed to load slos from definition files")
			os.Exit(1)
		}

		p := templates.NewPipeline(*buildName)
		for _, slo := range slos {
			logger.Log("event", "register_slo", "template", reflect.TypeOf(slo), "name", slo.GetName())
			p.MustRegister(slo)
		}

		groupsYaml, err := yaml.Marshal(p.Build())
		if err != nil {
			logger.Log("error", err, "msg", "failed to generate groups YAML")
			os.Exit(1)
		}

		os.Stdout.Write(groupsYaml)
	}
}

func loadDefinitions(definitionFiles []string) ([]templates.SLO, error) {
	slos := []templates.SLO{}
	for _, definitionFile := range definitionFiles {
		logger := kitlog.With(logger, "file", definitionFile)
		logger.Log("event", "parse_definitions")

		definition, err := ioutil.ReadFile(definitionFile)
		if err != nil {
			return nil, err
		}

		definitionSlos, err := templates.ParseDefinitions(definition)
		if err != nil {
			return nil, err
		}

		for _, slo := range definitionSlos {
			slos = append(slos, slo)
		}
	}

	return slos, nil
}

// Set by compilation process
var (
	Version   = "dev"
	Commit    = "none"
	Date      = "unknown"
	GoVersion = runtime.Version()
)

func versionStanza() string {
	return fmt.Sprintf(
		"slo-builder Version: %v\nGit SHA: %v\nGo Version: %v\nGo OS/Arch: %v/%v\nBuilt at: %v",
		Version, Commit, GoVersion, runtime.GOOS, runtime.GOARCH, Date,
	)
}
