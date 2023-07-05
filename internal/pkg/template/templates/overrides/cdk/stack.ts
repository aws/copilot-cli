import * as cdk from 'aws-cdk-lib';
import * as path from 'path';
{{- range $import := .Resources.Imports }}
import { {{$import.ImportName}} as {{$import.ImportShortRename}} } from 'aws-cdk-lib';
{{- end }}

interface TransformedStackProps extends cdk.StackProps {
    readonly appName: string;
    {{- if .RequiresEnv }}
    readonly envName: string;
    {{- end }}
}

export class TransformedStack extends cdk.Stack {
    public readonly template: cdk.cloudformation_include.CfnInclude;
    public readonly appName: string;
    {{- if .RequiresEnv }}
    public readonly envName: string;
    {{- end }}

    constructor (scope: cdk.App, id: string, props: TransformedStackProps) {
        super(scope, id, props);
        this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
            templateFile: path.join('.build', 'in.yml'),
        });
        this.appName = props.appName;
        {{- if .RequiresEnv }}
        this.envName = props.envName;
        {{- end }}

        {{- range $resource := .Resources }}
        this.transform{{$resource.LogicalID}}();
        {{- end }}
    }
    {{range $resource := .Resources}}
    // TODO: implement me.
    transform{{$resource.LogicalID}}() {
        const {{lowerInitialLetters $resource.LogicalID}} = this.template.getResource("{{$resource.LogicalID}}") as {{$resource.Type.ImportShortRename}}.{{$resource.Type.L1ConstructName}};
        throw new Error("not implemented");
    }
    {{end }}
}