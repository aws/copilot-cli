# Style Guide
* [Mission statement](#mission-statement)
* [Command grammar](#command-grammar)
* [Input](#input)
  * [Arguments](#arguments)
  * [Flags](#flags)
  * [Prompting](#prompting)
  * [Deleting and modifying production resources](#deleting-and-modifying-production-resources)
* [Autocomplete](#autocomplete)
* [Use full sentences](#use-full-sentences)
* [Stdout vs. Stderr](#stdout-vs-stderr)
* [Screen sizes](#screen-sizes)
* [Output](#output)
  * [Colors](#colors)
  * [Data](#data)
  * [List commands](#list-commands)
  * [Show commands](#show-commands)
  * [Progress](#progress)
  * [Recommended actions](#recommended-actions)
  * [Errors](#errors)
* [Acknowledgments](#acknowledgments)

## Mission statement
The AWS Copilot CLI prioritizes **human experiences** over machines. We value a **consistent** user experience across commands.
This means that similar things should be done in similar ways, but that dissimilar things should be done in different ways.

## Command grammar
Commands follow the style "noun verb" or only "verb". For example, `app ls` or `init`.
The noun represents an entity and should be singular. Command names are lowercase words or acronyms (no hyphens, colons, underscores).

We don’t nest commands beyond a single level. If you want to add an additional noun under an existing command,
create an additional command and use flags instead. For example, adding a service to an app
should be `svc init --app <name>` instead of `app add svc`.

Typical command structures are:

* Showing an individual resource: `copilot [entity] show [identifier] [options]`
* Listing all resources: `copilot [entity] ls [options]`
* Creating a new resource: `copilot [entity] init [identifier] [options]`
* Updating a resource: `copilot [entity] update [options]`
* Deleting a resource: `copilot [entity] delete [identifier...] [options]`

## Input
### Arguments
The rule-of-thumb is to use at most one argument per command. Commands with positional arguments introduces
additional cognitive load to the user as they have to remember the order of the arguments.

A command should have an argument when there is only one required input for it.
For example, `app init [name]`. All other inputs are present as flags as they're optional.
The argument’s name should be obvious to the user. If there is any possible confusion, the long description of the
command should bring clarity on what the input represents.

The exception to the rule is if you want to write a bulk command, where the type of the arguments are the same and
their order doesn’t matter, such as `copilot env delete test-pdx test-iad`.

### Flags
Flags are used when you need more than one required input of different type or if the inputs are optional.
Every flag needs to have a description and should [use full sentences](#use-full-sentences).
All flag names are lowercase and the same across commands. Long flag names can use hyphens if necessary to disambiguate.

Flags can also have a single character short name such as `env init -n MyEnv -a MyApp`.
Required flags must have a short name, only provide a short name to an optional flag if you think it will be frequently used.

Optional flags must have a default value. We want users to have the freedom to execute complex operations,
but by providing sane defaults we reduce the cognitive load to accomplish a task.

### Prompting
Prompt the user for any required flag that the user did not provide a value for. Prompting is a human-friendly way of showing options:
![prompt](https://user-images.githubusercontent.com/879348/65549185-e4394380-ded1-11e9-9fce-f1b19cd4f27f.png)

Don’t prompt the user if a flag value was provided to enable scripting.
Prompts should be written to `stderr`.
Highlight the important words in a prompt by bolding and italicizing the words.

### Deleting and modifying production resources
Any command that deletes or acts on a production resource should prompt the user to make sure they want to proceed with the action.
The flag `-y, --yes` should be present in these commands to skip confirmation.

## Autocomplete
Command names and flag names should be autocompleted using the <tab><tab> keys.

## Use full sentences
Guidance text for the user must use full sentences, begin with a capital letter, and end with punctuation.
For example, the description for `env init` is “Creates a new environment in your application.” and not “creates a new environment in your application”.
Similarly, when we prompt the user for additional information, we use full sentences.

## Stdout vs. Stderr
The console displays text written to both `stdout` and `stderr`. However, when you pipe the output of one command to another, only
`stdout` is passed over. Any data that can be used as input to a subsequent command should be written to `stdout`.
All other information should be written to `stderr`.

Typically, for listing or showing a resource we write a table to `stdout`. For mutation commands, we write the prompts and
diagnostic messages to `stderr` and at the end write the id of the resources created to `stdout`.

## Screen sizes
By default most terminals default to [80x24 character screens](https://softwareengineering.stackexchange.com/questions/148754/why-is-24-lines-a-common-default-terminal-height).
Break your sentences such that they don't go beyond 80 characters in a single line.
Otherwise, new line breaks might be oddly placed in screens of varying sizes.
While displaying [progress](#progress), don't show more than 24 lines at a time. Otherwise, the cursor
won't be able to move above 24 lines and you'll have truncated progress events.

## Output
### Colors
We use [colors](https://en.wikipedia.org/wiki/ANSI_escape_code#Colors) to set highlights — catching the user’s eye with important information.
We do not use color as the primary means of communication. Running without color support should not meaningfully degrade the UX.
For categorical data we use distinctive colors.

Common categorical data are debug, info, warning, success, failure messages.
We respectively use _white_, default, _yellow_, _bright green_ and _bright red_ for these messages.
For highlighting user input, use _cyan_.
For highlighting created resources, use _bring cyan_.
For highlighting follow-up commands, use _bring cyan_ and wrap the text with the back quote character (\`).

### Data
Commands that perform “read” operations should write their results to `stdout`.

If the data can be in a table format, make sure it's grep-parseable.
Display data in a [human-readable](https://github.com/dustin/go-humanize) format (friendly numbers, singular vs. plural, no ISO).
Commands that output tables should provide a `--json` flag and other possible formats like `--yaml` to display the raw response in JSON/YAML format.

```
$ copilot env ls
test-pdx
$ ecs env ls --json
{"environments":[{"app":"appName","name":"test","region":"us-west-2","accountID":"123456789012","prod":false,"registryURL":"","executionRoleARN":"arn:aws:iam::123456789012:role/buildspec-test-CFNExecutionRole","managerRoleARN":"arn:aws:iam::123456789012:role/buildspec-test-EnvManagerRole"}]}
```

If the command can listen on updates, then provide a `--follow` flag.

### List commands

For list commands like `app ls`, `env ls`, and `svc ls`, prefer formatting the output in a table. Include the name of the resource in the first column, and useful contextual metadata in additional columns. In general, prefer less information in list commands, as the amount of data shown grows with the number of entities being listed and can quickly become overwhelming.

```
$ copilot svc ls
Name                Type
--------            ---------------------
kudos-api           Load Balanced Web App
```

### Show commands

For show commands, like `app show`, `env show`, and `svc show`, include as many details as is useful, but try to logically organize the information into sections. In those sections use whatever format makes the most sense - tables for data that will have multiple rows, or key/value for non-row-based data. In the example `svc show` call below, the command has three logical sections: `About`, `Configurations`, and `Routes`. 'About' uses a key/value layout to describe the svc, while 'Configurations' and 'Routes' use tables, as that data varies by environment.

```
copilot svc show
About
  Application       ecs-kudos
  Name              api
  Type              Load Balanced Web App

Configurations
  Environment       Tasks               CPU (vCPU)          Memory (MiB)        Platform          Port
  test              1                   0.25                512                 LINUX/X86_64      80

Routes
  Environment       URL                                                              Path
  test              ecs-k-Publi-1Q9VOJGS3OQG1-100102515.us-west-2.elb.amazonaws.com  *
```

Prefer high-level summaries in show commands, rather than comprehensive views. Hiding less frequently useful data behind flags helps to not overwhelm users.

### Progress
Commands that perform actions (mutations) should write their updates to `stderr`.
Non-instantaneous operations must provide a signal to the user that it’s not stuck and still making progress on the task.

Use a spinner with a short text for short processes less than 4 seconds.

For long operations, use a spinner and display sub-tasks with status updates if the operation is going to take more than 4 seconds.
Prefer listing all sub-tasks up front.
![task-progress](https://user-images.githubusercontent.com/879348/65549555-9ffa7300-ded2-11e9-955f-fa842263fb99.gif)

Otherwise, display them sequentially over time.
![sequential-task-progress](https://user-images.githubusercontent.com/879348/65549577-abe63500-ded2-11e9-988b-00be9e770c85.gif)

### Recommended actions
Commands that are missing prerequisites should suggest other commands to run prior to invocation. For example:
```
$ copilot svc init
✘ could not find an application attached to this workspace, please run `app init` first
```

Commands that perform actions successfully should suggest additional steps to follow. For example:
```
$ copilot svc init -n frontend -t "Load Balanced Web Service" -d ./frontend/Dockerfile
✔ Success! Wrote the manifest for service frontend at 'copilot/frontend/manifest.yml'

Recommended follow-up actions:
- Update your manifest copilot/frontend/manifest.yml to change the defaults.
```

### Errors
Failure messages should be written to `stderr`.

```
✘ Failed! could not create directory "copilot": no permissions
```

Wrap external dependency errors to display errors as: `{what happened}: {why it happened}`

## Acknowledgments
Thanks to the following resources for their inspiration:
* [Heroku CLI’s style guide](https://devcenter.heroku.com/articles/cli-style-guide).
* Carolyn Van Slyck’s [Designing Command-Line Tools People Love](https://carolynvanslyck.com/talk/go/cli/#/) talk.
