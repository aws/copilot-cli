<div class="separator"></div>

<a id="logging" href="#logging" class="field">`logging`</a> <span class="type">Map</span>  
logging セクションには、ログ設定を含みます。このセクションでは、コンテナの [FireLens](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/using_firelens.html) ログドライバ用のパラメータを設定できます。(設定例は[こちら](../developing/sidecars.ja.md#sidecar-patterns))

<span class="parent-field">logging.</span><a id="retention" href="#logging-retention" class="field">`retention`</a> <span class="type">Integer</span>  
任意項目。 ログイベントを保持する日数。設定可能な値については、[こちら](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-logs-loggroup.html#cfn-logs-loggroup-retentionindays)を確認してください。省略した場合、デフォルトの 30 が設定されます。

<span class="parent-field">logging.</span><a id="logging-image" href="#logging-image" class="field">`image`</a> <span class="type">Map</span>  
任意項目。使用する Fluent Bit のイメージ。デフォルト値は `public.ecr.aws/aws-observability/aws-for-fluent-bit:stable`。

<span class="parent-field">logging.</span><a id="logging-destination" href="#logging-destination" class="field">`destination`</a> <span class="type">Map</span>  
任意項目。FireLens ログドライバーにログを送信するときの設定。

<span class="parent-field">logging.</span><a id="logging-enableMetadata" href="#logging-enableMetadata" class="field">`enableMetadata`</a> <span class="type">Map</span>  
任意項目。ログに ECS メタデータを含めるかどうか。デフォルトは `true`。

<span class="parent-field">logging.</span><a id="logging-secretOptions" href="#logging-secretOptions" class="field">`secretOptions`</a> <span class="type">Map</span>  
任意項目。ログの設定に渡す秘密情報です。

<span class="parent-field">logging.</span><a id="logging-configFilePath" href="#logging-configFilePath" class="field">`configFilePath`</a> <span class="type">Map</span>  
任意項目。カスタムの Fluent Bit イメージ内の設定ファイルのフルパス。

<span class="parent-field">logging.</span><a id="logging-envFile" href="#logging-envFile" class="field">`env_file`</a> <span class="type">String</span>  
ロギングサイドカーコンテナに設定する環境変数を含むファイルのワークスペースのルートからのパス。環境変数ファイルに関する詳細については、[環境変数ファイルの指定に関する考慮事項](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations)を確認してください。
