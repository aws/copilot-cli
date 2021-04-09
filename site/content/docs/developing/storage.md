# Storage

There are two ways to add persistence to Copilot workloads: using [`copilot storage init`](#database-and-artifacts) to create databases and S3 buckets; and attaching an existing EFS filesystem using the [`storage` field](#file-systems) in the manifest. 

## Database and Artifacts

To add a database or S3 bucket to your job or service, simply run [`copilot storage init`](../commands/storage-init.md).
```bash
# For a guided experience.
$ copilot storage init -t S3

# To create a bucket named "my-bucket" accessible by the "api" service.
$ copilot storage init -n my-bucket -t S3 -w api
```

The above command will create the Cloudformation template for an S3 bucket in the [addons](../developing/additional-aws-resources.md) directory for the "api" service. The next time you run `copilot deploy -n api`, the bucket will be created, permission to access it will be added to the `api` task role, and the name of the bucket will be injected into the `api` container under the environment variable `MY_BUCKET_NAME`. 

!!!info
    All names are converted into SCREAMING_SNAKE_CASE based on their use of hyphens or underscores. You can view the environment variables for a given service by running `copilot svc show`.

You can also create a [DynamoDB table](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Introduction.html) using `copilot storage init`. For example, to create the Cloudformation template for a table with a sort key and a local secondary index, you could run the following command.

```bash
# For a guided experience.
$ copilot storage init -t DynamoDB

# Or skip the prompts by providing flags.
$ copilot storage init -n users -t DynamoDB -w api --partition-key id:N --sort-key email:S --lsi post-count:N
```

This will create a DynamoDB table called `${app}-${env}-${svc}-users`. Its partition key will be `id`, a `Number` attribute; its sort key will be `email`, a `String` attribute; and it will have a [local secondary index](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/LSI.html) (essentially an alternate sort key) on the `Number` attribute `post-count`. 

It is also possible to create an [RDS Aurora Serverless](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless.html) cluster using `copilot storage init`. 
```bash
# For a guided experience.
$ copilot storage init -t Aurora

# Or skip the prompts by providing flags.
$ copilot storage init -n my-cluster -t Aurora -w api --engine PostgreSQL --initial-db my_db
```
This will create an RDS Aurora Serverless cluster that uses PostgreSQL engine with a database named `my_db`. An environment variable named `MYCLUSTER_SECRET` is injected into your workload as a JSON string. The fields are `'host'`, `'port'`, `'dbname'`, `'username'`, `'password'`, `'dbClusterIdentifier'` and `'engine'`.

## File Systems
Mounting an EFS volume in Copilot tasks requires two things:

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

Full syntax for storage follows. 

```yaml
storage:
  volumes:
    {{ volume name }}:
      path: {{ mount path }}             # Required. The path inside the container.
      read_only: {{ boolean }}           # Default: true
      efs:
        id: {{ filesystem ID }}          # Required.
        root_dir: {{ filesystem root }}  # Optional. Defaults to "/". Must not be 
                                         # specified if using access points.
        auth: 
          iam: {{ boolean }}             # Optional. Whether to use IAM authorization when 
                                         # mounting this filesystem.
          access_point_id: {{ access point ID}} # Optional. The ID of the EFS Access Point
                                                # to use when mounting this filesystem.
```

### Creating Mount Targets
There are several ways to create mount targets for an existing EFS filesystem: [using the AWS CLI](#with-the-aws-cli) and [using CloudFormation](#cloudformation).

#### With the AWS CLI
To create mount targets for an existing filesystem, you'll need 

1. the ID of that filesystem.
2. a Copilot environment deployed in the same account and region.

To retrieve the filesystem ID, you can use the AWS CLI:
```bash
$ EFS_FILESYSTEMS=$(aws efs describe-file-systems | \
  jq '.FileSystems[] | {ID: .FileSystemId, CreationTime: .CreationTime, Size: .SizeInBytes.Value}')
```

If you `echo` this variable you should be able to find which filesystem you need. Assign it to the variable `$EFS_ID` and continue.

You'll also need the public subnets of the Copilot environment and the Environment Security Group. This jq command will filter the output of the describe-stacks call down to simply the desired output value. 

!!!info
    The filesystem you use MUST be in the same region as your Copilot environment!

```bash
$ SUBNETS=$(aws cloudformation describe-stacks --stack-name ${YOUR_APP}-${YOUR_ENV} \
  | jq '.Stacks[] | .Outputs[] | select(.OutputKey == "PublicSubnets") | .OutputValue')
$ SUBNET1=$(echo $SUBNETS | jq -r 'split(",") | .[0]')
$ SUBNET2=$(echo $SUBNETS | jq -r 'split(",") | .[1]')
$ ENV_SG=$(aws cloudformation describe-stacks --stack-name ${YOUR_APP}-${YOUR_ENV} \
  | jq -r '.Stacks[] | .Outputs[] | select(.OutputKey == "EnvironmentSecurityGroup") | .OutputValue')
```

Once you have these, create the mount targets.

```bash
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

```bash
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
```bash
$ aws cloudformation deploy 
    --stack-name efs-cfn \
    --template-file ecs.yml
    --parameter-overrides App=${YOUR_APP} Env=${YOUR_ENV}
```

This will create an EFS file system and the mount targets your tasks need using outputs from the Copilot environment stack.

To get the EFS filesystem ID, you can run a `describe-stacks` call:

```bash
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
        id: {{ your filesystem ID }}
```

Finally, run `copilot svc deploy` to reconfigure your service to mount the filesystem at `/etc/mount1`. 

##### Cleanup
To clean this up, remove the `storage` configuration from the manifest and redeploy the service:
```bash
$ copilot svc deploy
```

Then, delete the stack. 

```bash
$ aws cloudformation delete-stack --stack-name efs-cfn
```

