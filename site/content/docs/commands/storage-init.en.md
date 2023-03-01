# storage init
```console
$ copilot storage init
```
## What does it do?
`copilot storage init` creates new storage resources as addons.

By default, Copilot follows the "database-per-service" pattern:
only the service or job that you specify during `storage init` will have access to that storage resource.
The storage is accessible from inside the service's containers via a friendly environment variable that
holds the name of your storage resource or credential information for accessing the resource.

!!!note ""
    However, each user has its own unique situation. If you do need your data storage to be shared among multiple service,
    you can modify the Copilot-generated CloudFormation template in order to achieve your goal.

A storage resource can be created as a [workload addon](../developing/addons/workload.en.md):
it is attached to one of your services or jobs, and is deployed and deleted at the same time as the workload.
For example, when you run `copilot svc deploy --name api`, the resource will be deployed along with "api"
to the target environment.

Alternatively, a storage resource can be created as an [environment addon](../developing/addons/environment.en.md):
it is attached to environments, and is deployed and deleted at the same time as an environment.
For example, when you run `copilot env deploy --name test`, the resource will be deployed along with the
"test" environment.

You can specify either *S3*, *DynamoDB* or *Aurora* as the resource type.


## What are the flags?
```
Required Flags
  -l, --lifecycle string      Whether the storage should be created and deleted
                              at the same time as an workload or an environment.
                              Must be one of: "workload" or "environment".
  -n, --name string           Name of the storage resource to create.
  -t, --storage-type string   Type of storage to add. Must be one of:
                              "DynamoDB", "S3", "Aurora".
  -w, --workload string       Name of the service/job that accesses the storage resource.

DynamoDB Flags
      --lsi stringArray        Optional. Attribute to use as an alternate sort key. May be specified up to 5 times.
                               Must be of the format '<keyName>:<dataType>'.
      --no-lsi                 Optional. Don't ask about configuring alternate sort keys.
      --no-sort                Optional. Skip configuring sort keys.
      --partition-key string   Partition key for the DDB table.
                               Must be of the format '<keyName>:<dataType>'.
      --sort-key string        Optional. Sort key for the DDB table.
                               Must be of the format '<keyName>:<dataType>'.

Aurora Serverless Flags
      --engine string               The database engine used in the cluster.
                                    Must be either "MySQL" or "PostgreSQL".
      --initial-db string           The initial database to create in the cluster.
      --parameter-group string      Optional. The name of the parameter group to associate with the cluster.
      --serverless-version string   Optional. Aurora Serverless version.
                                    Must be either "v1" or "v2" (default "v2").

Optional Flags
      --add-ingress-from string   The workload that needs access to an
                                  environment storage resource. Must be specified 
                                  with "--name" and "--storage-type".
                                  Can be specified with "--engine".
```

## How can I use it? 
Create an S3 bucket named "my-bucket" attached to the "frontend" service.

```console
$ copilot storage init -n my-bucket -t S3 -w frontend -l workload
```

Create an environment S3 bucket named "my-bucket" fronted by the "api" service.
```console
$ copilot storage init \
  -t S3 -n my-bucket \
  -w api -l environment
```

Create a basic DynamoDB table named "my-table" attached to the "frontend" service with a sort key specified.

```console
$ copilot storage init -t DynamoDB -n my-table \
  -w frontend -l workload \
  --partition-key Email:S \
  --sort-key UserId:N \
  --no-lsi
```

Create a DynamoDB table with multiple alternate sort keys.

```console
$ copilot storage init -t DynamoDB -n my-table \
  -w frontend \
  --partition-key Email:S \
  --sort-key UserId:N \
  --lsi Points:N \
  --lsi Goodness:N
```

Create an RDS Aurora Serverless v2 cluster using PostgreSQL as the database engine.
```console
$ copilot storage init \
  -n my-cluster -t Aurora -w frontend --engine PostgreSQL
```

Create an RDS Aurora Serverless v1 cluster using MySQL as the database engine with testdb as initial database name.
```console
$ copilot storage init \
  -n my-cluster -t Aurora --serverless-version v1 -w frontend --engine MySQL --initial-db testdb
```


## What happens under the hood?
Copilot writes a Cloudformation template specifying the S3 bucket, DDB table, or Aurora Serverless cluster to the `addons` dir. 
When you run `copilot [svc/job/env] deploy`, the CLI merges this template with all the other templates in the addons 
directory to create a nested stack associated with your service or environment. 
This nested stack describes all the [additional resources](../developing/addons/workload.en.md) you've associated with 
that service or the environment, and is deployed wherever your service or environment is deployed. 

### Example scenarios
#### S3 storage attached to a service
```console
$ copilot storage init --storage-type S3 --name bucket \
--workload fe --lifecycle workload
```
This generates a CloudFormation template for an S3 bucket, attached to the "fe" service.
```console
$ copilot svc deploy --name fe --env test
$ copilot svc deploy --name fe --env prod
```
After running these commands, there will be two buckets deployed,
one in the "test" env and one in the "prod" env,
accessible only to the "fe" service in its respective environment.

#### S3 storage attached to environments

```console
$ copilot storage init --storage-type S3 --name bucket \
--workload fe --lifecycle environment
```

This generates a CloudFormation template for an S3 bucket that will be deployed and deleted at the same time as an environment.
In addition, it will generate a CloudFormation template for the access policy that grants "fe" access
to the "bucket" resource.
```console
$ copilot env deploy --name test
$ copilot env deploy --name prod
```
After running the commands, there will be two buckets deployed, one each in the "test" and "prod" environments.

```console
$ copilot svc deploy --name fe --env test
$ copilot svc deploy --name fe --env prod
```

The service "fe" will be deployed with the access policy that is generated.
It is now able to access the S3 bucket in the respective environment.