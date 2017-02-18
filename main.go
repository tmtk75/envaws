package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/jawher/mow.cli"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/vaughan0/go-ini"
)

func main() {
	desc := `AWS access key manager
	    Help you to export environment variables and unset them easily.

   e.g)
     To export AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY for default profile

        $ eval $(envaws env default)

     To unset them

        $ eval $(envaws unset)`

	app := cli.App("envaws", desc)
	//app.Version = "0.1.0dev"

	app.Command("ls", "List available profiles in ~/.aws/credentials", func(c *cli.Cmd) {
		f := c.Bool(cli.BoolOpt{Name: "full f", Desc: "Print AccountId, ARN for each profile", Value: false})
		c.Spec = "[-f]"
		c.Action = func() {
			listProfiles(*f)
		}
	})
	app.Command("env", "Print keys as environtment variables for profile", func(c *cli.Cmd) {
		profile := c.String(cli.StringArg{Name: "PROFILE", Desc: "Profile name"})
		c.Action = func() {
			sec := loadParams(*profile)
			formatEnv(sec, "TBD")
		}
	})
	app.Command("unset", "Print commands to unset environtment variables for AWS_*", func(c *cli.Cmd) {
		c.Action = func() {
			unset()
		}
	})
	app.Command("tf", "Print keys as terraform variable definition for profile", func(c *cli.Cmd) {
		var (
			profile = c.String(cli.StringArg{Name: "PROFILE", Desc: "Profile name"})
			format  = c.String(cli.StringOpt{Name: "format f", Desc: "option,env,var,export", Value: "env"})
		)
		c.Spec = "PROFILE"
		c.Action = func() {
			sec := loadParams(*profile)
			formatTf(sec, *format)
		}
	})
	app.Command("role", "", func(c *cli.Cmd) {
		c.Command("ls", "", func(c *cli.Cmd) {
			f := c.Bool(cli.BoolOpt{Name: "full f", Desc: "Print RoleId, ARN for each profile", Value: false})
			c.Action = func() {
				svc := iam.New(session.New(), &aws.Config{})
				res, err := svc.ListRoles(nil)
				if err != nil {
					log.Fatalln(err)
				}

				w := new(tabwriter.Writer)
				w.Init(os.Stdout, 0, 8, 0, '\t', 0)
				for _, r := range res.Roles {
					if *f {
						fmt.Fprintf(w, "%v\t%v\t%v\n", *r.RoleName, *r.RoleId, *r.Arn)
					} else {
						fmt.Fprintf(w, "%v\n", *r.RoleName)
					}
				}
				w.Flush()
			}
		})
		c.Command("get", "", func(c *cli.Cmd) {
			rolename := c.String(cli.StringArg{Name: "ROLE_NAME", Desc: "Role name"})
			c.Action = func() {
				svc := iam.New(session.New(), &aws.Config{})
				res, err := svc.GetRole(&iam.GetRoleInput{RoleName: rolename})
				if err != nil {
					log.Fatalln(err)
				}
				b, _ := json.MarshalIndent(res, "", "  ")

				fmt.Println(string(b))
				d, _ := url.QueryUnescape(*res.Role.AssumeRolePolicyDocument)
				fmt.Println(d)
			}
		})
	})

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
unset AWS_SECURITY_TOKEN
unset AWS_DEFAULT_PROFILE
`
	//unset AWS_DEFAULT_REGION
	//unset AWS_DEFAULT_OUTPUT
	//unset AWS_CONFIG_FILE
	fmt.Println(strings.TrimSpace(templ))
}

type out struct {
	name string
	out  *sts.GetCallerIdentityOutput
	err  error
}

func listProfiles(full bool) {
	f := loadInifile("~/.aws/credentials")
	keys := make([]string, 0)
	for p, _ := range f {
		keys = append(keys, p)
	}
	sort.Strings(keys)

	if !full {
		for _, k := range keys {
			fmt.Println(k)
		}
		return
	}

	slots := make([]*out, len(keys))
	var wg sync.WaitGroup
	wg.Add(len(keys))
	for i, k := range keys {
		go func(i int, name string) {
			cred := credentials.NewSharedCredentials("", name)
			svc := sts.New(session.New(), &aws.Config{Credentials: cred})
			res, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
			slots[i] = &out{name: name, out: res, err: err}
			wg.Done()
		}(i, k)
	}
	wg.Wait()

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	for _, r := range slots {
		if r.err != nil {
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n", r.name, "", "", "", strings.Replace(r.err.Error(), "\n", " ", -1))
		} else {
			res := r.out
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\n", r.name, *res.Account, *res.UserId, *res.Arn)
		}
	}
	w.Flush()
}
