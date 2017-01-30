package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/tmtk75/cli"
	"github.com/vaughan0/go-ini"
)

func main() {
	app := cli.NewApp()
	app.Name = "envaws"
	app.Version = "0.1.0dev"
	app.Usage = `AWS access key manager
	    Help you to export environment variables and unset them easily.

   e.g)
     To export AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY for default profile

        $ eval $(envaws env default)

     To unset them

        $ eval $(envaws unset)`
	app.Commands = []cli.Command{
		cli.Command{
			Name:  "ls",
			Usage: "List available profiles in ~/.aws/credentials",
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) {
				listProfiles()
			},
		},
		cli.Command{
			Name:  "env",
			Usage: "Print keys as environtment variables for profile",
			Args:  "<profile>",
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) {
				profile, _ := ctx.ArgFor("profile")
				sec := loadParams(profile)
				formatEnv(sec, ctx.String("format"))
			},
		},
		cli.Command{
			Name:  "unset",
			Usage: "Print commands to unset environtment variables for AWS_*",
			Action: func(ctx *cli.Context) {
				unset()
			},
		},
		cli.Command{
			Name:  "tf",
			Args:  "<profile>",
			Usage: "Print keys as terraform variable definition for profile",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "format,f", Value: "var", Usage: "option,env,var,export"},
			},
			Action: func(ctx *cli.Context) {
				profile, _ := ctx.ArgFor("profile")
				sec := loadParams(profile)
				formatTf(sec, ctx.String("format"))
			},
		},
	}
	app.Run(os.Args)
}

func loadParams(profile string) ini.Section {
	sec := loadSection(profile)
	cnf := load("~/.aws/config", "profile "+profile, false)
	for k, v := range cnf {
		sec[k] = v
	}
	return sec
}

const ACCESS_KEY_ID = "aws_access_key_id"
const SECRET_ACCESS_KEY = "aws_secret_access_key"
const DEFAULT_REGION = "region"

func loadInifile(path string) ini.File {
	path, err := homedir.Expand(path)
	if err != nil {
		log.Fatalf("doesn't exist: %v\n", err)
	}

	if file, err := os.Open(path); err != nil {
		log.Fatalf("failed to open: %v\n", err)
	} else {
		inifile, err := ini.Load(file)
		if err != nil {
			log.Fatalf("%v\n", err)
		}

		return inifile
	}
	// Never reach
	return nil
}

func loadSection(profile string) ini.Section {
	return load("~/.aws/credentials", profile, true)
}

func load(path, profile string, check bool) ini.Section {
	inifile := loadInifile(path)
	for k, _ := range inifile {
		if k == profile {
			sec := inifile[k]
			if check {
				if sec[ACCESS_KEY_ID] == "" {
					log.Fatalf("aws_access_key_id is empty")
				}
				if sec[SECRET_ACCESS_KEY] == "" {
					log.Fatalf("aws_secret_access_key is empty")
				}
			}
			return sec
		}
	}
	if check {
		log.Fatalf("missing %v in %v\n", profile, path)
	}
	return map[string]string{}
}

const tf_env = `TF_VAR_aws_access_key="{{.aws_access_key_id}}" TF_VAR_aws_secret_key="{{.aws_secret_access_key}}"{{if .aws_region}} TV_VAR_region="{{.aws_region}}"{{end}}`
const tf_var = `
aws_access_key = "{{.aws_access_key_id}}"
aws_secret_key = "{{.aws_secret_access_key}}"
{{if .aws_region}}aws_region = "{{.aws_region}}"{{end}}
`
const tf_option = `-var aws_access_key="{{.aws_access_key_id}}" -var aws_secret_key="{{.aws_secret_access_key}}"{{if .aws_region}} -var region="{{.aws_region}}"{{end}}`
const tf_export = `
export TF_VAR_aws_access_key="{{.aws_access_key_id}}"
export TF_VAR_aws_secret_key="{{.aws_secret_access_key}}"
{{if .aws_region}}export TF_VAR_region="{{.aws_region}}"{{end}}`

func formatTf(sec ini.Section, format string) {
	templ := (map[string]string{
		"env":    tf_env,
		"var":    tf_var,
		"option": tf_option,
		"export": tf_export,
	})[format]
	if templ == "" {
		log.Fatalf("not supported format: %v", format)
	}

	formatTempl(sec, templ)
}

func formatEnv(sec ini.Section, format string) {
	templ := `
export AWS_ACCESS_KEY_ID="{{.aws_access_key_id}}"
export AWS_SECRET_ACCESS_KEY="{{.aws_secret_access_key}}"
{{if .aws_region}}export AWS_DEFAULT_REGION="{{.aws_region}}"{{end}}
`
	formatTempl(sec, templ)
}

func formatTempl(sec ini.Section, templ string) {
	vals := map[string]interface{}{
		"aws_access_key_id":     sec[ACCESS_KEY_ID],
		"aws_secret_access_key": sec[SECRET_ACCESS_KEY],
		"aws_region":            sec[DEFAULT_REGION],
	}
	t, _ := template.New("tf").Parse(templ)
	s := bytes.NewBuffer([]byte{})
	t.Execute(s, vals)
	fmt.Println(strings.TrimSpace(s.String()))

}

func unset() {
	templ := `
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
`
	//unset AWS_DEFAULT_REGION
	//unset AWS_DEFAULT_OUTPUT
	//unset AWS_DEFAULT_PROFILE
	//unset AWS_CONFIG_FILE
	//unset AWS_SECURITY_TOKEN
	fmt.Println(strings.TrimSpace(templ))
}

func listProfiles() {
	f := loadInifile("~/.aws/credentials")
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	for p, _ := range f {
		os.Setenv("AWS_PROFILE", p)
		svc := sts.New(session.New(), &aws.Config{})
		res, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
		if err != nil {
			fmt.Fprintf(w, "%v\n", p)
		} else {
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\n", p, *res.Account, *res.UserId, *res.Arn)
		}
	}
	w.Flush()
}
