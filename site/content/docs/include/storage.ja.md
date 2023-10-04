<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
`storage` セクションでは、コンテナやサイドカーでマウントしたい EFS ボリュームを指定できます。これにより、リージョン内のアベイラビリティゾーンにまたがって永続化ストレージへのアクセスが必要となるデータ処理や CMS のようなワークロードの実行が可能となります。詳細は[ストレージ](../developing/storage.ja.md)ページもご覧ください。また、タスクレベルのエフェメラルストレージの拡張を設定もできます。

<span class="parent-field">storage.</span><a id="ephemeral" href="#ephemeral" class="field">`ephemeral`</a> <span class="type">Int</span>
タスクに割り当てたいエフェメラルストレージのサイズを GiB で指定します。デフォルトかつ最小値は 20 GiB で、最大値は 200 GiB です。20 GiB を超えるサイズを指定した場合、サイズに応じた追加の料金が発生します。

タスクのメインコンテナとサイドカーでファイルシステムを共有したい場合、例えば次のように空ボリュームを使う方法が検討できます。
```yaml
storage:
  ephemeral: 100
  volumes:
    scratch:
      path: /var/data
      read_only: false

sidecars:
  mySidecar:
    image: public.ecr.aws/my-image:latest
    mount_points:
      - source_volume: scratch
        path: /var/data
        read_only: false
```
この例ではサイドカーとメインコンテナで共有されるボリュームとして、100 GiB のストレージがプロビジョンされます。例えば大きなサイズのデータセットを利用したい場合、あるいはディスク I/O の要求が高いワークロードにおいてサイドカーを利用して EFS からデータをコピーするような場合に有効な方法と言えます。

<span class="parent-field">storage.</span><a id="storage-readonlyfs" href="#storage-readonlyfs" class="field">`readonly_fs`</a> <span class="type">Boolean</span>
コンテナのルートファイルシステムに読み取り専用でアクセス出来る様にするには、true を指定します。

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
マウントしたい EFS ボリュームの名前や設定を指定します。`volumes` フィールドでは次のように Map を利用して指定します。
```yaml
volumes:
  <volume name>:
    path: "/etc/mountpath"
    efs:
      ...
```

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`<volume>`</a> <span class="type">Map</span>  
ボリュームの設定を指定します。

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
必須設定項目です。ボリュームをマウントするコンテナ内のパスを指定します。指定する値は２４２文字未満かつ `a-zA-Z0-9.-_/` の文字種である必要があります。

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
任意設定項目で、デフォルト値は `true` です。ボリュームを読み取り専用とするかどうかを指定します。`false` に設定した場合、コンテナにファイルシステムへの `elasticfilesystem:ClientWrite` 権限が付与され、それによりボリュームへ書き込めるようになります。

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Boolean or Map</span>  
詳細な EFS 設定を指定します。Boolean 値による指定、あるいは `uid` と `gid` サブフィールドのみを指定した場合に、EFS ファイルシステムと Service 専用の EFS アクセスポイントが作成されます。

```yaml
// Boolean 値を指定する場合
efs: true

// POSIX uid/gid を指定する場合
efs:
  uid: 10000
  gid: 110000
```

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
必須設定項目です。マウントする EFS ファイルシステムの ID を指定します。

<span class="parent-field">storage.volumes.`<volume>`.efs.id.</span><a id="from_cfn" href="#from_cfn" class="field">`from_cfn`</a> <span class="type">String</span> <span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>  
[CloudFormation スタック出力値のエクスポート](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html)の名前を指定します。

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>
任意設定項目で、デフォルト値は `/` です。EFS ファイルシステム内のどのパスをマウントするボリュームのルートとするのかを指定します。指定する値は 255 文字未満かつ `a-zA-Z0-9.-_/` の文字種である必要があります。EFS アクセスポイントを利用する場合、本設定値に空もしくは `/` を指定し、かつ `auth.iam` の設定値が `true` となっている必要があります。

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="uid" href="#uid" class="field">`uid`</a> <span class="type">Uint32</span>
任意設定項目で、`gid` とともに指定する必要があります。また、`root_dir`、`auth`、`id` とともに指定することはできません. Copilot 管理の EFS ファイルシステムに対する EFS アクセスポイントを作成する際の POSIX UID として利用されます。

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="gid" href="#gid" class="field">`gid`</a> <span class="type">Uint32</span>
任意設定項目で、`uid` とともに指定する必要があります。また、`root_dir`、`auth`、`id` とともに指定することはできません. Copilot 管理の EFS ファイルシステムに対する EFS アクセスポイントを作成する際の POSIX GID として利用されます。

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
EFS に関連する認可設定を指定します。

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Boolean</span>  
任意設定項目で、デフォルトは `true` です。EFS リソースへのアクセスに IAM による認可を利用するかどうかを指定します。

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
任意設定項目で、デフォルトは `""` です。利用する EFS アクセスポイントの ID を指定します。EFS アクセスポイントを利用する場合、`root_dir` の設定値に空もしくは `/` を指定しており、かつ本設定値が `true` となっている必要があります。
