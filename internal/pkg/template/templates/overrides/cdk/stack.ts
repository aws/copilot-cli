import * as cdk from 'aws-cdk-lib';
import * as path from 'path';
{{- range $resourceType := .Resources.UniqueTypes }}
import { {{$resourceType.ImportName}} as {{$resourceType.ImportShortRename}} } from 'aws-cdk-lib';
{{- end }}

interface TransformedStackProps extends cdk.StackProps {
    readonly appName: string;
    readonly envName: string;
}

export class TransformedStack extends cdk.Stack {
    constructor (scope: cdk.App, id: string, props: TransformedStackProps) {
        super(scope, id, props);
        this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
            templateFile: path.join('.build', 'in.yml'),
        });
        this.appName = props.appName;
        this.envName = props.envName;

        {{- range $resource := .Resources }}
        this.transform{{$resource.LogicalID}}();
        {{- end }}
    }
    {{range $resource := .Resources}}
    // TODO: implement me.
    transform{{$resource.LogicalID}}() {
        const {{lowerInitialLetters $resource.LogicalID}} = this.template.getResource("{{$resource.LogicalID}}") as {{$resource.Type.ImportShortRename}}.{{$resource.Type.L1ConstructName}};
        throw new error("not implemented");
    }
    {{end }}
}