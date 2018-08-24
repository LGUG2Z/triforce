# triforce
[![Go Report Card](https://goreportcard.com/badge/github.com/lgug2z/triforce)](https://goreportcard.com/report/github.com/lgug2z/triforce)
[![Maintainability](https://api.codeclimate.com/v1/badges/2296d2da4304647c9bcc/maintainability)](https://codeclimate.com/github/LGUG2Z/triforce/maintainability)
[![Test Coverage](https://api.codeclimate.com/v1/badges/2296d2da4304647c9bcc/test_coverage)](https://codeclimate.com/github/LGUG2Z/triforce/test_coverage)
[![Build Status](https://travis-ci.org/LGUG2Z/triforce.svg?branch=master)](https://travis-ci.org/LGUG2Z/triforce)

`triforce` assembles and links `node` dependencies across meta and monorepo projects.

## Installation and Quickstart
### Binary
```bash
go get -u github.com/LGUG2Z/triforce

cd /your/meta/or/mono/repo
triforce assemble --exclude whatever .

npm install

triforce link .
```

### Docker Image
```
docker pull lgug2z/triforce:latest
docker run --workdir /tmp --volume ${HOME}/projects:/tmp lgug2z/triforce:latest assemble .

npm install

docker run --workdir /tmp --volume ${HOME}/projects:/tmp lgug2z/triforce:latest link .
```

### Docker-Compose
```yaml
version: '3.6'

services:
  assemble:
    image: lgug2z/triforce
    volumes:
      - /your/meta/or/mono/repo:/tmp
    working_dir: /tmp
    command: assemble .

  link:
    image: lgug2z/triforce
    volumes:
      - /your/meta/or/mono/repo:/tmp
    working_dir: /tmp
    command: link .
```

### Example Docker CI Image
If you want to run a scheduled CI job that can assemble all of your project
dependencies, and have the required apt dependencies to install them and
compile any native extensions they might require:
```bash
docker pull lgug2z/triforce:ci
```

## When this can be useful
### Multi projects with their own package.json file
Imagine you have a codebase that consists of multiple different node projects,
and that each of these projects has its own `package.json` file which requires
installing for the project to be developed.

```bash
example-metarepo
├── api-1
│   └── package.json
├── api-2
│   └── package.json
├── api-3
│   └── package.json
├── app-1
│   └── package.json
├── app-2
│   └── package.json
├── app-3
│   └── package.json
├── lib-1
│   └── package.json
├── lib-2
│   └── package.json
└── lib-3
    └── package.json
```

Now imagine that your libraries are so tighly coupled with your api or app
projects that it is never possible to implement a new feature in an api or
an app without making changes in one or more of the underlying libraries.


### Too many projects for an 'npm link'-driven workflow
As changes will need to be pushed to multiple projects at the same time to
ensure successful builds and deployments, there is a need for a local
development flow that allows for development against changes in those libraries
that are still on the local or remote development machine. This is usually where
`npm link` comes in for smaller scale projects. But what if you have tens of
libraries? And apis? And apps? It quickly becomes unwieldy.

### Too many projects for a 'zelda'-driven workflow
At this point you may be able to turn to a tool like [zelda](https://github.com/feross/zelda),
which takes advantage of the way that `node` tries to look for dependencies on a filesystem.
Image one of your projects is checked out at `/Users/ponyo/dev/example-metarepo/lib-1`. `node`
will try to resolve the dependencies for this project by looking for a `node_modules` folder
containing the required dependency in every directory up the tree all the way to `/node_modules`:

```bash
# looking for dependency 'lib-2'

/Users/ponyo/dev/example-metarepo/lib-1/node_modules/lib-2 # doesn't exist, let's try -> |
#                                                                                        |
# |--------------------------------------<--------------------------------------------<- |
# v
/Users/ponyo/dev/example-metarepo/node_modules/lib-2 # doesn't exist, let's try -> |
#                                                                                  |
# |-----------------------------------<------------------------------------------<-|
# v
/Users/ponyo/dev/node_modules/lib-2 # doesn't exist, let's try -> |
#                                                                 |
# |------------------------------<------------------------------<-|
# v
/Users/ponyo/node_modules/lib-2 # exists! let's use it!
```

The problem, however, is that `zelda` is not very efficient; it will install dependencies
of a project in its `package.json` files, including any private dependencies referenced with VCS
URLs, then look one level up to see if it has installed any dependencies with names matching projects
that live in the parent folder. If any are found, the dependency as installed in the project
`node_modules` folder will be removed, and then a separate `npm install` will be run in the checked out
copy of the dependent project. And this process will keep happening recursively until there is nothing
else left to clean up and reinstall in a checked out project in the parent folder. Finally, a symlink
`node_modules` is created in the parent folder that points to itself, leaving you with this:

```bash
lrwxr-xr-x    1 ponyo  staff     1 11 Jul 16:07 node_modules -> .
```

This is great, because any project can use a local version of a private dependency for development
purposes and they can all be checked in together when the time is right. However, running `zelda`
individually on tens of repositories becomes incredibly slow and can take many hours just to do
an initial installation to get a codebase up and running for the first time. Not to mention, every
subsequent time `zelda` is run, every public dependency that was previously downloaded gets blown
away.

### triforce workflows

`triforce` ultimately has two tasks:
* assemble all of the `dependencies` and `devDependencies` across a collection of projects into a single 
`package.json` file
* create symlinks of any checked out private dependencies within the top-level `node_dependencies` folder 
which contains dependencies from the assembled `package.json` file

These tasks are completed with two commands:
```bash
triforce assemble ~/my/meta/or/mono/repo
```

```bash
# after the assembled package.json file has been installed
triforce link ~/my/meta/or/mono/repo
```

They leave you with something resembling:

```bash
example-metarepo
├── api-1
├── app-1
├── lib-1
└── node_modules
    ├── acl 
    ├── api-1 -> ../api-1
    ├── app-1 -> ../app-1
    ├── lib-1 -> ../lib-1
    ├── lodash
    ├── node-sass
    └── react
```

#### Dependency versions
When dealing with a codebase comprised of a large number of `node` projects, it will almost always be the
case that different projects will require ever so slightly different versions of the same dependency, or
that the same dependency will be a listed as a `dependency` in one project and a `devDependency` in another.
For now, `triforce` deals with this in a simple way:
* If the same dependency is listed with different versions across projects, pick the highest version
* If the same dependency is listed as a `dependency` and a `devDependency` across projects, promote it to 
a `dependency` with the higher version


### Excluding private dependencies
`triforce` by default excludes any dependencies where the version contains `bitbucket`, `github` or `gitlab`.
Additional exclusions can be specified by using the `--exclude` flag when running the `assemble` command:

```bash
triforce assemble --exclude MySecretGithubOrgName ~/path/to/my/meta/or/mono/repo
```

### Making developer onboarding even faster
`triforce` can be used to take a `zelda` workflow that takes ~5 hours for an initial install across an
entire codebase down to 20 minutes. Not bad, but still not great. If a team develops in a Dockerised
development environment, then it is already a given that everyone is running against the same version
of `node` on the same operating system and distro.

With this in mind, a task can be scheduled to run on a CI platform that checks out the latest version
of a meta or monorepo, runs `triforce assemble` and then compresses and uploads the resulting `node_modules`
folder to something like S3 or GCS, where developers can fetch the latest dump of installed dependencies
from every morning.
