package cli_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"fmt"

	"github.com/LGUG2Z/triforce/cli"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type BasicPackageJSON struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type TestSpace struct {
	RootFolder string
	Projects   map[string]*BasicPackageJSON
}

func (t *TestSpace) Destroy() error {
	if t != nil {
		if err := os.RemoveAll(t.RootFolder); err != nil {
			return err
		}

		if err := os.RemoveAll("package.json"); err != nil {
			return err
		}
	}

	return nil
}

func NewTestSpace(projects map[string]*BasicPackageJSON) (*TestSpace, error) {
	t := &TestSpace{RootFolder: "ginkgo_tests", Projects: projects}
	for project, pkg := range projects {
		if err := os.MkdirAll(filepath.Join(t.RootFolder, project), os.FileMode(0700)); err != nil {
			return nil, err
		}

		bytes, err := json.MarshalIndent(pkg, "", "  ")
		if err != nil {
			return nil, err
		}

		if err := ioutil.WriteFile(filepath.Join(t.RootFolder, project, "package.json"), bytes, os.FileMode(0700)); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Join(t.RootFolder, "node_modules"), os.FileMode(0700)); err != nil {
		return nil, err
	}

	return t, nil
}

var _ = Describe("Assemble", func() {
	var p map[string]*BasicPackageJSON
	var t *TestSpace
	var err error

	BeforeEach(func() {
		p = make(map[string]*BasicPackageJSON)
	})

	AfterEach(func() {
		Expect(t.Destroy()).To(Succeed())
	})

	Context("sanity checking", func() {
		It("should throw an error if trying to assemble from a root directory that doesn't exist", func() {
			args := []string{"triforce", "assemble", "this-dir-does-not-exist"}
			Expect(cli.App().Run(args)).NotTo(Succeed())
		})

		It("should throw an error if trying to assemble without enough args", func() {
			args := []string{"triforce", "assemble"}
			Expect(cli.App().Run(args)).NotTo(Succeed())
		})

		It("should throw an error if trying to assemble from broken package.json files", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().Build()
			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(filepath.Join(t.RootFolder, "project-1", "package.json"), []byte("not json"), os.FileMode(0700))).To(Succeed())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).NotTo(Succeed())
		})
	})

	Context("projects with dependencies containing the default exclusion patterns for private dependencies", func() {
		It("should exclude private dependencies from BitBucket, GitHub and GitLab from the triforce package.json", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().Dependency("dep-a", "github.com/someorg/dep-a.git").Build()
			p["project-2"] = NewBasicPackageJSONBuilder().
				Dependency("dep-b", "1.0.0").
				Dependency("dep-c", "bitbucket.com/someorg/dep-c.git").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.Dependencies)).To(Equal(1))
		})
	})

	Context("projects with dependencies containing custom exclusion patterns for private dependencies", func() {
		It("should exclude private dependencies matching a custom exclusion pattern from the triforce package.json", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().DevDependency("devdep-a", "excluded/dep-a.git").Build()
			p["project-2"] = NewBasicPackageJSONBuilder().Dependency("dep-b", "1.0.0").Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", "--exclude", "excluded", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.Dependencies)).To(Equal(1))
			Expect(len(pkg.DevDependencies)).To(Equal(0))
		})
	})

	Context("projects with no overlapping dependencies or devDependencies", func() {
		It("should give an automated name and description to the triforce package.json", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				Dependency("dep-a", "1.0.0").
				DevDependency("devdev-a", "1.0.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				Dependency("dep-b", "1.0.0").
				DevDependency("devdep-b", "1.0.0").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(pkg.Name).To(Equal("triforce-ginkgo_tests"))
			Expect(pkg.Description).To(Equal("automatically generated by triforce"))
		})

		It("should contain all dependencies and devDependencies", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				Dependency("dep-a", "1.0.0").
				DevDependency("devdev-a", "1.0.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				Dependency("dep-b", "1.0.0").
				DevDependency("devdep-b", "1.0.0").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.Dependencies)).To(Equal(2))
			Expect(len(pkg.Dependencies)).To(Equal(2))
		})

	})

	Context("projects with overlapping dependencies", func() {
		It("should only have one entry for overlapping dependencies in the triforce package.json", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				Dependency("dep-a", "1.0.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				Dependency("dep-a", "2.0.0").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.Dependencies)).To(Equal(1))

			By("selecting the dependency with the highest version number", func() {
				Expect(pkg.Dependencies).To(HaveKeyWithValue("dep-a", "2.0.0"))
			})
		})

		It("should skip if trying to add the same dependency with a lower version", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				Dependency("dep-a", "1.0.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				Dependency("dep-a", "0.2.0").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.Dependencies)).To(Equal(1))

			By("skipping the dependency with the lower version", func() {
				Expect(pkg.Dependencies).To(HaveKeyWithValue("dep-a", "1.0.0"))
			})
		})
	})

	Context("projects with overlapping devDependencies", func() {
		It("should only have one entry for overlapping devDependencies in the triforce package.json", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				DevDependency("devdep-a", "~1.0.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				DevDependency("devdep-a", "~1.0.1").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.DevDependencies)).To(Equal(1))

			By("selecting the devDependency with the highest version number", func() {
				Expect(pkg.DevDependencies).To(HaveKeyWithValue("devdep-a", "~1.0.1"))
			})
		})

		It("should skip if trying to add the same devDependency with a lower version", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				DevDependency("devdep-a", "^1.0.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				DevDependency("devdep-a", "~0.0.1").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.DevDependencies)).To(Equal(1))

			By("skipping the devDependency with the lower version number", func() {
				Expect(pkg.DevDependencies).To(HaveKeyWithValue("devdep-a", "^1.0.0"))
			})
		})

	})

	Context("projects with overlapping dependencies and devDependencies", func() {
		It("should be listed only once as a dependency, not as a devDependency, in the triforce package.json", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				Dependency("somelib", "^2.5.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				DevDependency("somelib", "^2.6.1").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.DevDependencies)).To(Equal(0))
			Expect(len(pkg.Dependencies)).To(Equal(1))

			By("selecting the dependency with the highest version number", func() {
				Expect(pkg.Dependencies).To(HaveKeyWithValue("somelib", "^2.6.1"))
			})
		})

		It("should skip the devDependency if the previously added dependency is of a higher version", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				Dependency("somelib", "^2.7.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				DevDependency("somelib", "^2.6.1").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "assemble", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())
			Expect("package.json").To(BeAnExistingFile())

			bytes, err := ioutil.ReadFile("package.json")
			Expect(err).NotTo(HaveOccurred())
			pkg := BasicPackageJSON{}
			Expect(json.Unmarshal(bytes, &pkg)).To(Succeed())

			Expect(len(pkg.DevDependencies)).To(Equal(0))
			Expect(len(pkg.Dependencies)).To(Equal(1))

			By("skipping the dependency with the lower version number", func() {
				Expect(pkg.Dependencies).To(HaveKeyWithValue("somelib", "^2.7.0"))
			})

		})
	})
})

var _ = Describe("Link", func() {
	var p map[string]*BasicPackageJSON
	var t *TestSpace
	var err error

	BeforeEach(func() {
		p = make(map[string]*BasicPackageJSON)
	})

	AfterEach(func() {
		Expect(t.Destroy()).To(Succeed())
	})

	Context("sanity checking", func() {
		It("should throw an error if trying to link from a root directory that doesn't exist", func() {
			args := []string{"triforce", "link", "this-dir-does-not-exist"}
			Expect(cli.App().Run(args)).NotTo(Succeed())
		})

		It("should throw an error if trying to link without enough args", func() {
			args := []string{"triforce", "link"}
			Expect(cli.App().Run(args)).NotTo(Succeed())
		})

		It("should throw an error if trying to link from a root directory that doesn't contain installed node_modules", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				DevDependency("somelib", "^2.6.1").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.RemoveAll(filepath.Join(t.RootFolder, "node_modules"))).To(Succeed())

			args := []string{"triforce", "link", t.RootFolder}
			Expect(cli.App().Run(args)).NotTo(Succeed())
		})
	})

	Context("projects with their dependencies installed in the meta/mono project root directory", func() {
		It("should link the private projects within the existing node_modules folder", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				Dependency("dep", "^2.5.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				DevDependency("devdep", "^2.6.1").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "link", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())

			for _, project := range []string{"project-1", "project-2"} {
				expectedSymlink := filepath.Join(t.RootFolder, "node_modules", project)
				Expect(expectedSymlink).To(BeADirectory())

				symlinkOrigin, err := os.Readlink(expectedSymlink)
				Expect(err).NotTo(HaveOccurred())
				Expect(symlinkOrigin).To(Equal(fmt.Sprintf("../%s", project)))
			}
		})

		It("should remove existing symlinks before trying to create new ones", func() {
			p["project-1"] = NewBasicPackageJSONBuilder().
				Dependency("dep", "^2.5.0").
				Build()

			p["project-2"] = NewBasicPackageJSONBuilder().
				DevDependency("devdep", "^2.6.1").
				Build()

			t, err = NewTestSpace(p)
			Expect(err).NotTo(HaveOccurred())

			args := []string{"triforce", "link", t.RootFolder}
			Expect(cli.App().Run(args)).To(Succeed())

			// run again to make sure we handle the case where symlinks already exist
			Expect(cli.App().Run(args)).To(Succeed())

			for _, project := range []string{"project-1", "project-2"} {
				expectedSymlink := filepath.Join(t.RootFolder, "node_modules", project)
				Expect(expectedSymlink).To(BeADirectory())

				symlinkOrigin, err := os.Readlink(expectedSymlink)
				Expect(err).NotTo(HaveOccurred())
				Expect(symlinkOrigin).To(Equal(fmt.Sprintf("../%s", project)))
			}
		})
	})
})

type BasicPackageJSONBuilder struct {
	basicPackageJSON *BasicPackageJSON
}

func NewBasicPackageJSONBuilder() *BasicPackageJSONBuilder {
	basicPackageJSON := &BasicPackageJSON{Dependencies: make(map[string]string), DevDependencies: make(map[string]string)}
	b := &BasicPackageJSONBuilder{basicPackageJSON: basicPackageJSON}
	return b
}

func (b *BasicPackageJSONBuilder) Dependency(dependency, version string) *BasicPackageJSONBuilder {
	b.basicPackageJSON.Dependencies[dependency] = version
	return b
}

func (b *BasicPackageJSONBuilder) DevDependency(devDependency, version string) *BasicPackageJSONBuilder {
	b.basicPackageJSON.DevDependencies[devDependency] = version
	return b
}

func (b *BasicPackageJSONBuilder) Build() *BasicPackageJSON {
	return b.basicPackageJSON
}
