<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
The Storage section lets you specify external EFS volumes for your containers and sidecars to mount. This allows you to access persistent storage across availability zones in a region for data processing or CMS workloads. For more detail, see the [storage](../developing/storage.en.md) page. You can also specify extensible ephemeral storage at the task level.

<span class="parent-field">storage.</span><a id="ephemeral" href="#ephemeral" class="field">`ephemeral`</a> <span class="type">Int</span>  
Specify how much ephemeral task storage to provision in GiB. The default value and minimum is 20 GiB. The maximum size is 200 GiB. Sizes above 20 GiB incur additional charges.

To create a shared filesystem context between an essential container and a sidecar, you can use an empty volume:
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
This example will provision 100 GiB of storage to be shared between the sidecar and the task container. This can be useful for large datasets, or for using a sidecar to transfer data from EFS into task storage for workloads with high disk I/O requirements.

<span class="parent-field">storage.</span><a id="storage-readonlyfs" href="#storage-readonlyfs" class="field">`readonly_fs`</a> <span class="type">Boolean</span>
Specify true to give your container read-only access to its root file system.

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
Specify the name and configuration of any EFS volumes you would like to attach. The `volumes` field is specified as a map of the form:
```yaml
volumes:
  <volume name>:
    path: "/etc/mountpath"
    efs:
      ...
```

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`<volume>`</a> <span class="type">Map</span>  
Specify the configuration of a volume.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
Required. Specify the location in the container where you would like your volume to be mounted. Must be fewer than 242 characters and must consist only of the characters `a-zA-Z0-9.-_/`.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
Optional. Defaults to `true`. Defines whether the volume is read-only or not. If false, the container is granted `elasticfilesystem:ClientWrite` permissions to the filesystem and the volume is writable.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Boolean or Map</span>  
Specify more detailed EFS configuration. If specified as a boolean, or using only the `uid` and `gid` subfields, creates a managed EFS filesystem and dedicated Access Point for this workload.

```yaml
// Simple managed EFS
efs: true

// Managed EFS with custom POSIX info
efs:
  uid: 10000
  gid: 110000
```

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
Required. The ID of the filesystem you would like to mount.

<span class="parent-field">storage.volumes.`<volume>`.efs.id.</span><a id="from_cfn" href="#from_cfn" class="field">`from_cfn`</a> <span class="type">String</span> <span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>  
The name of a [CloudFormation stack export](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html).

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
Optional. Defaults to `/`. Specify the location in the EFS filesystem you would like to use as the root of your volume. Must be fewer than 255 characters and must consist only of the characters `a-zA-Z0-9.-_/`. If using an access point, `root_dir` must be either empty or `/` and `auth.iam` must be `true`.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="uid" href="#uid" class="field">`uid`</a> <span class="type">Uint32</span>  
Optional. Must be specified with `gid`. Mutually exclusive with `root_dir`, `auth`, and `id`. The POSIX UID to use for the dedicated access point created for the managed EFS filesystem.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="gid" href="#gid" class="field">`gid`</a> <span class="type">Uint32</span>  
Optional. Must be specified with `uid`. Mutually exclusive with `root_dir`, `auth`, and `id`. The POSIX GID to use for the dedicated access point created for the managed EFS filesystem.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
Specify advanced authorization configuration for EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Boolean</span>  
Optional. Defaults to `true`. Whether or not to use IAM authorization to determine whether the volume is allowed to connect to EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
Optional. Defaults to `""`. The ID of the EFS access point to connect to. If using an access point, `root_dir` must be either empty or `/` and `auth.iam` must be `true`.
