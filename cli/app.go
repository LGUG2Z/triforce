package cli

import (
	"strings"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"path"

	"github.com/Jeffail/gabs"
	"github.com/fatih/color"
	"github.com/urfave/cli"
)

type TriforcePackageJSON struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

const PackageJSON = "package.json"
const NodeModules = "node_modules"

func App() *cli.App {
	app := cli.NewApp()
	app.Name = "triforce"
	app.Usage = "assembles and links node dependencies across meta and monorepo projects"
	app.UsageText = "triforce command [command options] [arguments...]"
	app.HideVersion = true
	app.Compiled = time.Now()
	app.Authors = []cli.Author{{
		Name:  "J. Iqbal",
		Email: "jade@beamery.com",
	}}

	app.Commands = []cli.Command{
		Assemble(),
		Link(),
	}

	return app
}

func Assemble() cli.Command {
	return cli.Command{
		Name:      "assemble",
		ShortName: "a",
		Usage:     "assembles the dependencies and devDependencies across all projects into a single package.json file",
		Flags: []cli.Flag{
			cli.StringSliceFlag{Name: "exclude, e", Usage: "patterns to exclude in versions", Value: &cli.StringSlice{"github", "gitlab", "bitbucket"}},
			cli.StringSliceFlag{Name: "filter, f", Usage: "patterns to include in projects", Value: &cli.StringSlice{}},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("triforce assemble requires a root meta or monorepo folder as an argument")
			}

			root, err := filepath.Abs(c.Args().First())
			if err != nil {
				return err
			}

			exclude := c.StringSlice("exclude")
			filter := c.StringSlice("filter")

			var parsedPackageJSONs []*gabs.Container
			dependencies := make(map[string]string)
			devDependencies := make(map[string]string)

			projectDirectories, err := getProjectFolders(root, filter)
			if err != nil {
				return err
			}

			projectDependencyMap := make(map[*gabs.Container]string)

			for _, projectDirectory := range projectDirectories {
				pkgPath := filepath.Join(root, projectDirectory, PackageJSON)
				if _, err := os.Stat(pkgPath); err == nil {
					parsed, err := gabs.ParseJSONFile(pkgPath)
					if err != nil {
						return err
					}

					parsedPackageJSONs = append(parsedPackageJSONs, parsed)
					projectDependencyMap[parsed] = path.Base(projectDirectory)
				}
			}

			for _, parsed := range parsedPackageJSONs {
				extractDependencies(projectDependencyMap[parsed], parsed, dependencies, exclude)
			}

			for _, parsed := range parsedPackageJSONs {
				extractDevDependencies(projectDependencyMap[parsed], parsed, dependencies, devDependencies, exclude)
			}

			t := TriforcePackageJSON{
				Name:            fmt.Sprintf("triforce-%s", filepath.Base(root)),
				Description:     fmt.Sprintf("automatically generated by triforce"),
				Dependencies:    dependencies,
				DevDependencies: devDependencies,
			}

			bytes, err := json.MarshalIndent(t, "", "  ")
			if err != nil {
				return err
			}

			return ioutil.WriteFile("package.json", bytes, os.FileMode(0666))
		},
	}
}

func Link() cli.Command {
	return cli.Command{
		Name:      "link",
		ShortName: "l",
		Usage:     "links private projects inside of the node_modules folder at the meta or monorepo project root",
		Flags: []cli.Flag{
			cli.StringSliceFlag{Name: "filter, f", Usage: "patterns to include in projects", Value: &cli.StringSlice{}},
		},
		Action: cli.ActionFunc(func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("triforce link requires a root meta or monorepo folder as an argument")
			}

			root, err := filepath.Abs(c.Args().First())
			if err != nil {
				return err
			}

			filter := c.StringSlice("filter")

			nodeModules := filepath.Join(root, NodeModules)
			if _, err := os.Stat(nodeModules); err != nil {
				return fmt.Errorf("no node_modules folder found at %s", root)
			}

			projectFolders, err := getProjectFolders(root, filter)
			if err != nil {
				return err
			}

			for _, f := range projectFolders {
				// if it is a node project
				pkgPath := filepath.Join(root, f, PackageJSON)
				if _, err := os.Stat(pkgPath); err == nil {
					projectDirectory := path.Join("..", f)
					symlinkDestination := filepath.Join(nodeModules, f)

					// remove symlinks if they already exist
					if _, err := os.Lstat(symlinkDestination); err == nil {
						if err := os.Remove(symlinkDestination); err != nil {
							return err
						}
					}

					if err := os.Symlink(projectDirectory, symlinkDestination); err != nil {
						return err
					}
					fmt.Printf("symlinked %s to %s\n", f, fmt.Sprintf("./node_modules/%s", f))
				}
			}

			color.Green("finished linking private dependencies to ./node_modules")

			return nil
		}),
	}
}

func getProjectFolders(root string, filters []string) ([]string, error) {
	var projectDirectories []string
	dirs, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
	}

	hasFilterPatterns := len(filters) > 0

	for _, d := range dirs {
		// ignore non-directories and hidden files
		if d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
			if hasFilterPatterns {
				for _, filter := range filters {
					if strings.Contains(d.Name(), filter) {
						projectDirectories = append(projectDirectories, d.Name())
						continue
					}
				}
			} else {
				projectDirectories = append(projectDirectories, d.Name())
			}
		}
	}

	return projectDirectories, nil
}

func isAPrivateDependency(version string, exclude ...string) bool {
	for _, pattern := range exclude {
		if strings.Contains(strings.ToLower(version), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

func extractDependencies(project string, parsed *gabs.Container, dependencies map[string]string, exclude []string) {
	if data, ok := parsed.Path("dependencies").Data().(map[string]interface{}); ok {
		if len(data) > 0 {
			color.Green("\nassembling dependencies from %s", project)
		}

		for dep, version := range data {
			if isAPrivateDependency(version.(string), exclude...) {
				color.Red(excluded("dependency", dep, version.(string)))
				continue
			}

			// Update in dependencies if it is a greater version
			if val, ok := dependencies[dep]; ok {
				if shouldUpdate(val, version.(string)) {
					dependencies[dep] = version.(string)
					fmt.Println(updated("dependency", dep, val, version.(string)))
					continue
				}
				color.Yellow(skipped("dependency", dep, version.(string), val))
				continue
			} else {
				// Otherwise add for the first time
				dependencies[dep] = version.(string)
				fmt.Println(added("dependency", dep, version.(string)))
			}
		}
	}
}

func extractDevDependencies(project string, parsed *gabs.Container, dependencies, devDependencies map[string]string, exclude []string) {
	if data, ok := parsed.Path("devDependencies").Data().(map[string]interface{}); ok {
		if len(data) > 0 {
			color.Green("\nassembling devDependencies from %s", project)
		}

		for devDep, version := range data {
			if isAPrivateDependency(version.(string), exclude...) {
				color.Red(excluded("devDependency", devDep, version.(string)))
				continue
			}

			// Update in dependencies if it is a greater version
			if val, ok := dependencies[devDep]; ok {
				if shouldUpdate(val, version.(string)) {
					dependencies[devDep] = version.(string)
					fmt.Println(promoted(devDep, val, version.(string)))
					continue
				}
				color.Yellow(skipped("devDependency", devDep, version.(string), val))
				continue
			}

			// Otherwise update in devDependencies if the version is greater
			if val, ok := devDependencies[devDep]; ok {
				if shouldUpdate(val, version.(string)) {
					devDependencies[devDep] = version.(string)
					fmt.Println(updated("devDependency", devDep, val, version.(string)))
					continue
				}
				color.Yellow(skipped("devDependency", devDep, version.(string), val))
				continue
			} else {
				// Otherwise add for the first time
				devDependencies[devDep] = version.(string)
				fmt.Println(added("devDependency", devDep, version.(string)))
			}
		}
	}
}

func shouldUpdate(original, new string) bool {
	original = strings.TrimPrefix(original, "^")
	original = strings.TrimPrefix(original, "~")

	new = strings.TrimPrefix(new, "^")
	new = strings.TrimPrefix(new, "~")

	return new > original
}

func skipped(depType, name, lowerVersion, higherVersion string) string {
	return fmt.Sprintf("skipped %s \"%s\" (previously assembled with equal or higher version \"%s\" > \"%s\")", depType, name, higherVersion, lowerVersion)
}

func excluded(depType, name, version string) string {
	return fmt.Sprintf("excluded %s \"%s\" (version \"%s\" matches exclusion patterns)", depType, name, version)
}

func added(depType, name, version string) string {
	return fmt.Sprintf("added %s \"%s\" with version \"%s\"", depType, name, version)
}

func updated(depType, name, lowerVersion, higherVersion string) string {
	return fmt.Sprintf("updated %s \"%s\" to higher version (\"%s\" > \"%s\")", depType, name, higherVersion, lowerVersion)
}

func promoted(name, lowerVersion, higherVersion string) string {
	return fmt.Sprintf("promoted devDependency \"%s\" to replace previously added dependency with higher version (\"%s\" > \"%s\")", name, higherVersion, lowerVersion)
}
