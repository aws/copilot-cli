# Contributing to the CLI

Thanks for your interest in contributing to the Amazon ECS CLI! 💖

This document describes how to set up a development environment and submit your contributions. Please read it over and let us know if it's not up-to-date (or, even better, submit a PR with your corrections 😉).

- [Development setup](#development-setup)
  - [Environment](#environment)
  - [Set upstream](#set-upstream)
  - [Building and testing](#building-and-testing)
  - [Generating mocks](#generating-mocks)
  - [Adding new dependencies](#adding-new-dependencies)
- [Where should I start?](#where-should-i-start)
- [Contributing code](#contributing-code)
- [Amazon Open Source Code of Conduct](#amazon-open-source-code-of-conduct)
- [Licensing](#licensing)

## Development setup

### Environment

- Make sure you are using Go 1.13 (`go version`).
- Fork the repository.
- Clone your forked repository locally.
- We use Go Modules to manage depenencies, so you can develop outside of your $GOPATH.

#### Set upstream

From the repository root run:

`git remote add upstream git@github.com:aws/amazon-ecs-cli-v2`

`git fetch upstream`

### Building and testing

There are three different types of testing done on the ECS CLI.

**Unit tests** makes up the majority of the testing and new code should include unit tests. Ideally, these unit tests will be in the same package as the file they're testing and have full coverage (or as much is practical within a unit test). Unit tests shouldn't make any network calls.

**Integration tests** are rarer and test the CLI's integration with remote services, such as CloudFormation or SSM. Our integration tests ensure that we can call these remote services and get the results we expect.

**End to End tests** run the CLI in a container and test the actual commands - including spinning and tearing down remote resources (like ECS clusters and VPCs). These tests are the most comprehensive and run on both Windows and Linux build fleets. Feel free to run these tests - but they require two AWS accounts to run in, so be mindful that resources will be created and destroyed. You'll need three [profiles](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html): `default`, `e2etestenv` and `e2eprodenv`. Each e2e profile needs to be configured for a different AWS account and a different region than the other e2e profile.

Below are the different commands which can be run in the root of the project directory.

* Run `make` (This creates a standalone executable in the `bin/local` directory).
* Run `make test` to run the unit tests.
* Run `make integ-test` to run integration tests against your Default AWS profile. **Warning** - this will create AWS resources in your account.
* Run `make e2e` to run end to end tests (tests that run commands locally). **Warning** - this will create AWS resources in your account. You'll need Docker running for these tests to run.

### Generating mocks
Often times it's helpful to generate mocks to make unit-testing easier and more focused. We strongly encourage this and encourage you to generate mocks when appropriate! In order to generate mocks:

* Add the package your interface is in under the [gen-mocks](https://github.com/aws/amazon-ecs-cli-v2/blob/master/Makefile#L43) command in the Makefile.
* run `make gen-mocks`

## Adding new dependencies

In general, we discourage adding new dependencies to the ECS CLI. If there's a module you think the CLI could benefit from, first open a PR with your proposal. We'll evaluate the dependency and the use case and decide on the next steps.

## Where should I start

We're so excited you want to contribute to the ECS CLI! We welcome all PRs and will try to get to them as soon as possible. The best place to start, though, is with filing an issue first. Filing an issue gives us some time to chat about the code you're keen on writing, and make sure that it's not already being worked on, or has already been discussed.

You can also check out our [issues queue](https://github.com/aws/amazon-ecs-cli-v2/issues) to see all the known issues - this is a really great place to start.

If you want to get your feet wet, check out issues tagged with [good first issue](https://github.com/aws/amazon-ecs-cli-v2/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22). These issues are great for folks who are looking to get started, but not sure where to start 😁.

## Contributing code
* Please check the existing issues to see if your feedback has already been reported.

* Let us know if you are interested in working on an issue by leaving a comment
on the issue in GitHub. This helps avoid multiple people unknowingly working on
the same issue.

* If you would like to propose a new feature, please open an issue on GitHub with
a detailed description. This enables us to collaborate on the feature design
more easily and increases the chances that your feature request will be accepted.

* New features should include full test coverage.

* New files should include the standard license  header.

* All submissions, including submissions by project members, require review. We
  use GitHub pull requests for this purpose. Consult GitHub Help for more
information on using pull requests.

## Amazon Open Source Code of Conduct

This code of conduct provides guidance on participation in Amazon-managed open source communities, and outlines the process for reporting unacceptable behavior. As an organization and community, we are committed to providing an inclusive environment for everyone. Anyone violating this code of conduct may be removed and banned from the community.

**Our open source communities endeavor to:**
* Use welcoming and inclusive language;
* Be respectful of differing viewpoints at all times;
* Accept constructive criticism and work together toward decisions;
* Focus on what is best for the community and users.

**Our Responsibility.** As contributors, members, or bystanders we each individually have the responsibility to behave professionally and respectfully at all times. Disrespectful and unacceptable behaviors include, but are not limited to:
The use of violent threats, abusive, discriminatory, or derogatory language;
* Offensive comments related to gender, gender identity and expression, sexual orientation, disability, mental illness, race, political or religious affiliation;
* Posting of sexually explicit or violent content;
* The use of sexualized language and unwelcome sexual attention or advances;
* Public or private [harassment](http://todogroup.org/opencodeofconduct/#definitions) of any kind;
* Publishing private information, such as physical or electronic address, without permission;
* Other conduct which could reasonably be considered inappropriate in a professional setting;
* Advocating for or encouraging any of the above behaviors.

**Enforcement and Reporting Code of Conduct Issues.**
Instances of abusive, harassing, or otherwise unacceptable behavior may be reported by contacting opensource-codeofconduct@amazon.com. All complaints will be reviewed and investigated and will result in a response that is deemed necessary and appropriate to the circumstances.

**Attribution.** _This code of conduct is based on the [template](http://todogroup.org/opencodeofconduct) established by the [TODO Group](http://todogroup.org/) and the Scope section from the [Contributor Covenant version 1.4](http://contributor-covenant.org/version/1/4/)._

## Licensing
The Amazon ECS CLI is released under an [Apache 2.0](http://aws.amazon.com/apache-2-0/) license. Any code you submit will be released under that license.

For significant changes, we may ask you to sign a [Contributor License Agreement](http://en.wikipedia.org/wiki/Contributor_License_Agreement).
