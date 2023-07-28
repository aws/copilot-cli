# Storage

There are two ways to add persistence to Copilot workloads: using [`copilot storage init`](#database-and-artifacts) to create databases and S3 buckets; and attaching an existing EFS filesystem using the [`storage` field](#file-systems) in the manifest.

## Database and Artifacts

To add a database or S3 bucket to your service, job, or environment, simply run [`copilot storage init`](../commands/storage-init.en.md).
```console
# For a guided experience.
$ copilot storage init -t S3

# To create a bucket named "my-bucket" that is accessible by the "api" service, and is deployed and deleted with "api".
$ copilot storage init -n my-bucket -t S3 -w api -l workload
```

The above command will create the Cloudformation template for an S3 bucket in the [addons](./addons/workload.en.md) directory for the "api" service.
The next time you run `copilot deploy -n api`, the bucket will be created, permission to access it will be added to the `api` task role,
and the name of the bucket will be injected into the `api` container under the environment variable `MY_BUCKET_NAME`.

!!!info
    All names are converted into SCREAMING_SNAKE_CASE based on their use of hyphens or underscores. You can view the environment variables for a given service by running `copilot svc show`.

You can also create a [DynamoDB table](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Introduction.html) using `copilot storage init`.
For example, to create the Cloudformation template for a table with a sort key and a local secondary index, you could run the following command:

```console
# For a guided experience.
$ copilot storage init -t DynamoDB

# Or skip the prompts by providing flags.
$ copilot storage init -n users -t DynamoDB -w api -l workload --partition-key id:N --sort-key email:S --lsi post-count:N
```

This will create a DynamoDB table called `${app}-${env}-${svc}-users`. Its partition key will be `id`, a `Number` attribute; its sort key will be `email`, a `String` attribute; and it will have a [local secondary index](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/LSI.html) (essentially an alternate sort key) on the `Number` attribute `post-count`.

It is also possible to create an [RDS Aurora Serverless v2](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless-v2.html) cluster using `copilot storage init`.
```console
# For a guided experience.
$ copilot storage init -t Aurora

# Or skip the prompts by providing flags.
$ copilot storage init -n my-cluster -t Aurora -w api -l workload --engine PostgreSQL --initial-db my_db
```
This will create an RDS Aurora Serverless v2 cluster that uses PostgreSQL engine with a database named `my_db`. An environment variable named `MYCLUSTER_SECRET` is injected into your workload as a JSON string. The fields are `'host'`, `'port'`, `'dbname'`, `'username'`, `'password'`, `'dbClusterIdentifier'` and `'engine'`.

To create an [RDS Aurora Serverless v1](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless.html) cluster, you can run
```console
$ copilot storage init -n my-cluster -t Aurora --serverless-version v1
```

### Environment storage

The `-l` flag is short for `--lifecycle`. In the examples above, the value to the `-l` flag is `workload`.
This means that the storage resources will be created as a service addon or a job addon. The storage will be deployed
when you run `copilot [svc/job] deploy`, and will be deleted when you run `copilot [svc/job] delete`.

Alternatively, if you want your storage to persist even after you delete the service or the job, you can create
an environment storage resource. An environment storage resource is created as an environment addon: it is deployed when you run
`copilot env deploy`, and isn't deleted until you run `copilot env delete`.

## File Systems
There are two ways to use an EFS file system with Copilot: using managed EFS, and importing your own filesystem.

!!! Attention
    EFS is not supported for Windows-based services.

### Managed EFS
The easiest way to get started using EFS for service- or job-level storage is via Copilot's built-in managed EFS capability. To get started, simply enable the `efs` key in the manifest under your volume's name.
```yaml
name: frontend

storage:
  volumes:
    myManagedEFSVolume:
      efs: true
      path: /var/efs
      read_only: false
```

This manifest will result in an EFS volume being created at the environment level, with an Access Point and dedicated directory at the path `/frontend` in the EFS filesystem created specifically for your service. Your container will be able to access this directory and all its subdirectories at the `/var/efs` path in its own filesystem. The `/frontend` directory and EFS filesystem will persist until you delete your environment. The use of an access point for each service ensures that no two services can access each other's data.

You can also customize the UID and GID used for the access point by specifying the `uid` and `gid` fields in advanced EFS configuration. If you do not specify a UID or GID, Copilot picks a pseudorandom UID and GID for the access point based on the [CRC32 checksum](https://stackoverflow.com/a/14210379/5890422) of the service's name. 

```yaml
storage:
  volumes:
    myManagedEFSVolume:
      efs: 
        uid: 1000
        gid: 10000
      path: /var/efs
      read_only: false
```

`uid` and `gid` may not be specified with any other advanced EFS configuration.

#### Under the Hood
When you enable managed EFS, Copilot creates the following resources at the environment level:

* An [EFS file system](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-efs-filesystem.html).
* [Mount targets](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-efs-mounttarget.html) in each of your environment's private subnets
* Security group rules allowing the Environment Security Group to access the mount targets. 

At the service level, Copilot creates:

* An [EFS Access Point](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-efs-accesspoint.html). The Access Point refers to a directory created by CFN named after the service or job you wish to use EFS with. 

You can see the environment-level resources created by calling `copilot env show --json --resources` and parsing the output with your favorite command line JSON processor. For example:
```
> copilot env show -n test --json --resources | jq '.resources[] | select( .type | contains("EFS") )'
```

#### Advanced Use Cases
##### Hydrating a Managed EFS Volume
Sometimes, you may wish to populate the created EFS volume with data before your service begins accepting traffic. There are several ways you can do this, depending on your main container's requirements and whether it requires this data for startup. 

###### Using a Sidecar
You can mount the created EFS volume in a sidecar using the [`mount_points`](../developing/sidecars.en.md) field, and use your sidecar's `COMMAND` or `ENTRYPOINT` directives to copy data from the sidecar's filesystem or pull data down from S3 or another cloud service. 

If you mark the sidecar as nonessential with `essential:false`, it will start, do its work, and exit as the service containers come up and stabilize. 

This may not be suitable for workloads which depend on the correct data being present in the EFS volume. 

###### Using `copilot svc exec`
For workloads where data must be present prior to your task containers coming up, we recommend using a placeholder container first. 

For example, deploy your `frontend` service with the following values in the manifest:
```yaml
name: frontend
type: Load Balanced Web Service

image:
  location: amazon/amazon-ecs-sample
exec: true

storage:
  volumes:
    myVolume:
      efs: true
      path: /var/efs
      read_only: false
```

Then, when your service is stable, run:
```console
$ copilot svc exec
```
This will open an interactive shell from which you can add packages like `curl` or `wget`, download data from the internet, create a directory structure, etc.

!!!info 
    This method of configuring containers is not recommended for production environments; containers are ephemeral and if you wish for a piece of software to be present in your service containers, be sure to add it using the `RUN` directive in a Dockerfile. 

When you have populated the directory, modify your manifest to remove the `exec` directive and update the `build` field to your desired Docker build config or image location.

```yaml
name: frontend
type: Load Balanced Web Service

image:
  build: ./Dockerfile
storage:
  volumes:
    myVolume:
      efs: true
      path: /var/efs
      read_only: false
```

### External EFS
Mounting an externally-managed EFS volume in Copilot tasks requires two things:

1. That you create an [EFS file system](https://docs.aws.amazon.com/efs/latest/ug/whatisefs.html) in the desired environment's region.
2. That you create an [EFS Mount Target](https://docs.aws.amazon.com/efs/latest/ug/accessing-fs.html) using the Copilot environment security group in each subnet of your environment.

When those prerequisites are satisfied, you can enable EFS storage using simple syntax in your manifest. You'll need the filesystem ID and, if using, the access point configuration for the filesystem.

!!!info
    You can only use a given EFS file system in a single environment at a time. Mount targets are limited to one per availability zone; therefore, you must delete any existing mount targets before bringing the file system to Copilot if you have used it in another VPC.

### Manifest Syntax
The simplest possible EFS volume can be specified with the following syntax:

```yaml
storage:
  volumes:
    myEFSVolume: # This is a variable key and can be set to an arbitrary string.
      path: '/etc/mount1'
      efs:
        id: fs-1234567
```

This will create a read-only mounted volume in your service's or job's container using the filesystem `fs-1234567`. If mount targets are not created in the subnets of the environment, the task will fail to launch.

Full syntax for external EFS volumes follows.

```yaml
storage:
  volumes:
    <volume name>:
      path: <mount path>             # Required. The path inside the container.
      read_only: <boolean>           # Default: true
      efs:
        id: <filesystem ID>          # Required.
        root_dir: <filesystem root>  # Optional. Defaults to "/". Must not be
                                         # specified if using access points.
        auth:
          iam: <boolean>             # Optional. Whether to use IAM authorization when
                                         # mounting this filesystem.
          access_point_id: <access point ID> # Optional. The ID of the EFS Access Point
                                                # to use when mounting this filesystem.
        uid: <uint32>                # Optional. UID for managed EFS access point.
        gid: <uint32>                # Optional. GID for managed EFS access point. Cannot be specified
                                     # with `id`, `root_dir`, or `auth`. 
```

### Creating Mount Targets
There are several ways to create mount targets for an existing EFS filesystem: [using the AWS CLI](#with-the-aws-cli) and [using CloudFormation](#cloudformation).

#### With the AWS CLI
To create mount targets for an existing filesystem, you'll need

1. the ID of that filesystem.
2. a Copilot environment deployed in the same account and region.

To retrieve the filesystem ID, you can use the AWS CLI:
```console
$ EFS_FILESYSTEMS=$(aws efs describe-file-systems | \
  jq '.FileSystems[] | {ID: .FileSystemId, CreationTime: .CreationTime, Size: .SizeInBytes.Value}')
```

If you `echo` this variable you should be able to find which filesystem you need. Assign it to the variable `$EFS_ID` and continue.

You'll also need the public subnets of the Copilot environment and the Environment Security Group. This jq command will filter the output of the describe-stacks call down to simply the desired output value.

!!!info
    The filesystem you use MUST be in the same region as your Copilot environment!

```console
$ SUBNETS=$(aws cloudformation describe-stacks --stack-name ${YOUR_APP}-${YOUR_ENV} \
  | jq '.Stacks[] | .Outputs[] | select(.OutputKey == "PublicSubnets") | .OutputValue')
$ SUBNET1=$(echo $SUBNETS | jq -r 'split(",") | .[0]')
$ SUBNET2=$(echo $SUBNETS | jq -r 'split(",") | .[1]')
$ ENV_SG=$(aws cloudformation describe-stacks --stack-name ${YOUR_APP}-${YOUR_ENV} \
  | jq -r '.Stacks[] | .Outputs[] | select(.OutputKey == "EnvironmentSecurityGroup") | .OutputValue')
```

Once you have these, create the mount targets.

```console
$ MOUNT_TARGET_1_ID=$(aws efs create-mount-target \
    --subnet-id $SUBNET_1 \
    --security-groups $ENV_SG \
    --file-system-id $EFS_ID | jq -r .MountTargetID)
$ MOUNT_TARGET_2_ID=$(aws efs create-mount-target \
    --subnet-id $SUBNET_2 \
    --security-groups $ENV_SG \
    --file-system-id $EFS_ID | jq -r .MountTargetID)
```

Once you've done this, you can specify the `storage` configuration in the manifest as above.

##### Cleanup

Delete the mount targets using the AWS CLI.

```console
$ aws efs delete-mount-target --mount-target-id $MOUNT_TARGET_1
$ aws efs delete-mount-target --mount-target-id $MOUNT_TARGET_2
```

#### CloudFormation
Here's an example of how you might create the appropriate EFS infrastructure for an external file system using a CloudFormation stack.

After creating an environment, deploy the following CloudFormation template into the same account and region as the environment.

Place the following CloudFormation template into a file called `efs.yml`.

```yaml
Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.

Resources:
  EFSFileSystem:
    Metadata:
      'aws:copilot:description': 'An EFS File System for persistent backing storage for tasks and services'
    Type: AWS::EFS::FileSystem
    Properties:
      PerformanceMode: generalPurpose
      ThroughputMode: bursting
      Encrypted: true

  MountTargetPublicSubnet1:
    Type: AWS::EFS::MountTarget
    Properties:
      FileSystemId: !Ref EFSFileSystem
      SecurityGroups:
        - Fn::ImportValue:
            !Sub "${App}-${Env}-EnvironmentSecurityGroup"
      SubnetId: !Select
        - 0
        - !Split
            - ","
            - Fn::ImportValue:
                !Sub "${App}-${Env}-PublicSubnets"

  MountTargetPublicSubnet2:
    Type: AWS::EFS::MountTarget
    Properties:
      FileSystemId: !Ref EFSFileSystem
      SecurityGroups:
        - Fn::ImportValue:
            !Sub "${App}-${Env}-EnvironmentSecurityGroup"
      SubnetId: !Select
        - 1
        - !Split
            - ","
            - Fn::ImportValue:
                !Sub "${App}-${Env}-PublicSubnets"
Outputs:
  EFSVolumeID:
    Value: !Ref EFSFileSystem
    Export:
      Name: !Sub ${App}-${Env}-FilesystemID
```

Then run:
```console
$ aws cloudformation deploy
    --stack-name efs-cfn \
    --template-file ecs.yml
    --parameter-overrides App=${YOUR_APP} Env=${YOUR_ENV}
```

This will create an EFS file system and the mount targets your tasks need using outputs from the Copilot environment stack.

To get the EFS filesystem ID, you can run a `describe-stacks` call:

```console
$ aws cloudformation describe-stacks --stack-name efs-cfn | \
    jq -r '.Stacks[] | .Outputs[] | .OutputValue'
```

Then, in the manifest of the service which you would like to have access to the EFS filesystem, add the following configuration.

```yaml
storage:
  volumes:
    copilotVolume: # This is a variable key and can be set to arbitrary strings.
      path: '/etc/mount1'
      read_only: true # Set to false if your service needs write access.
      efs:
        id: <your filesystem ID>
```

Finally, run `copilot svc deploy` to reconfigure your service to mount the filesystem at `/etc/mount1`.

##### Cleanup
To clean this up, remove the `storage` configuration from the manifest and redeploy the service:
```console
$ copilot svc deploy
```

Then, delete the stack.

```console
$ aws cloudformation delete-stack --stack-name efs-cfn
```
