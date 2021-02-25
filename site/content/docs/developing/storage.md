# Storage
All Copilot workloads which use a manifest can mount externally created EFS volumes using the `storage` field. 

## Adding EFS storage to Copilot
Mounting an EFS volume in Copilot tasks requires two things:

1. That you create an EFS file system in the region of the environment you wish to use it with
2. That you create an EFS Mount Target using the Copilot environment security group in each subnet of your environment. 

When those prerequisites are satisfied, you can enable EFS storage using simple syntax in your manifest. You'll need the filesystem ID and, if using, the access point configuration for the filesystem.

!!!info
    You can only use a given EFS file system in a single environment at a time. Mount targets are limited to one per availability zone; therefore, you must delete any existing mount targets before bringing the file system to Copilot if you have used it in another way. 

### Manifest Syntax
The simplest possible EFS volume can be specified with the following syntax:

```yaml
storage:
  volumes:
    myEFSVolume: # This is a variable key and can be set to arbitrary strings.
      path: '/etc/mount1'
      efs:
        id: fs-1234567 
```

This will create a read-only mounted volume in your service or job's container using the filesystem `fs-1234567`. If mount targets are not created in the subnets of the environment, the task will fail to launch. 

Full syntax for storage follows. 

```yaml
storage:
  volumes:
    {{ volume name }}:
      path: {{ path at which to mount }} # Required.
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

#### Using the AWS CLI
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

```bash
$ SUBNETS=$(aws cloudformation describe-stacks --stack-name pdx-app-test \
  | jq '.Stacks[] | .Outputs[] | select(.OutputKey == "PublicSubnets") | .OutputValue')
$ SUBNET1=$(echo $SUBNETS | jq -r 'split(",") | .[0]')
$ SUBNET2=$(echo $SUBNETS | jq -r 'split(",") | .[1]')
$ ENV_SG=$(aws cloudformation describe-stacks --stack-name pdx-app-test \
  | jq -r '.Stacks[] | .Outputs[] | select(.OutputKey == "EnvironmentSecurityGroup") | .OutputValue')
```

Once you have these, creating mount targets is simple. 
```bash
$ aws efs create-mount-target --subnet-id $SUBNET_1 --security-groups $ENV_SG --file-system-id $EFS_ID
$ aws efs create-mount-target --subnet-id $SUBNET_2 --security-groups $ENV_SG --file-system-id $EFS_ID
```

#### Using Addons
Here's an example of how you might create the appropriate EFS infrastructure for an external file system using the [Addons](../developing/additional-aws-resources.md) functionality. 

In a Copilot workspace, create a [Scheduled Job](../manifest/scheduled-job.md) which will never run. We'll use this to deploy our addons template which holds the mount targets we need without worrying about incurring charges for other infrastructure. 

```bash
$ copilot job init -n efs-job --schedule "cron(0 0 21 10 ? 2015)" -i amazon/amazon-ecs-sample 
$ copilot job deploy -n efs-job
```

From the root of your workspace, create the addons directory and a file for the EFS infrastructure to live in. 

```bash
$ mkdir copilot/efs-job/addons && touch copilot/efs-job/addons/efs.yaml
```
Add the following CloudFormation template in `efs.yaml`. 

```yaml
Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.
  Name:
    Type: String
    Description: The name of the service, job, or workflow being deployed.

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

```

Then, deploy your scheduled job again: 
```bash
$ copilot job deploy -n efs-job
```

This will create an EFS file system and the mount targets needed to allow tasks to attach to it. You can get the ID of this filesystem with the AWS CLI and jq.
```bash
$ aws efs describe-file-systems | \
  jq -r '.FileSystems[] | select((.Tags[]|select(.Key=="copilot-service")|.Value) =="efs-helper") | .FileSystemId'
```

Then, in the manifest of the service which you would like to have access to the EFS filesystem, add the following configuration.

```yaml
storage:
  volumes:
    copilotVolume: # This is a variable key and can be set to arbitrary strings.
      path: '/etc/mount1'
      read_only: true # Set to false if your service needs write access. 
      efs:
        id: {{ output of describe-file-systems }}
```

Finally, run `copilot svc deploy` to reconfigure your service to mount the filesystem at `/etc/mount1`. 

To clean this up, do one of the following options: 

1. Remove the `storage` configuration from the manifest and redeploy, then run `copilot job delete -n efs-helper`.
2. Delete the service, then delete the app: `copilot svc delete && copilot app delete`. 