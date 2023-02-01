<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Copilot は Service 名などを常に環境変数としてタスクに対して渡します。本フィールドではそれら以外に追加で渡したい環境変数をキーバーリューのペアで指定します。

<span class="parent-field">variables.</span><a id="variables-from-cfn" href="#variables-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
[CloudFormation スタックエクスポート](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html)の名称。

<div class="separator"></div>

<a id="env_file" href="#env_file" class="field">`env_file`</a> <span class="type">String</span>  
ワークスペースのルートから、メインコンテナに引き渡す環境変数を含むファイルへのパスを指定します。環境変数ファイルの詳細については、[環境変数ファイルの指定に関する考慮事項](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations)を参照してください。
