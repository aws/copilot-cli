# storage init
```console
$ copilot storage init
```
## What does it do?
`copilot storage init` creates a new storage resource as addons.

By default, Copilot follows the "database-per-service" pattern:
only the service or job that you specify during `storage init` will have access to that storage resource.
The storage is accessible from inside the service's containers via a friendly environment variable.

!!!note ""
	However, each product has its own unique situation. If you do need your data storage to be shared by multiple service,
	you can modify the CloudFormation template that Copilot generates for you to achieve your goal.

A storage can be created as a [workload addon](../developing/addons/workload.en.md):
it is attached to one of your services or jobs, and is deployed and deleted at the same time as the workload.
For example, when you run `copilot svc deploy --name api`, the storage resource will be deployed along with "api"
to the target environment.

Alternatively, a storage can be created as an [environment addon](../developing/addons/environment.en.md):
it is attached to environments, and is deployed and deleted at the same time as an environment.
For example, when you run `copilot env deploy --name test`, the storage resource will be deployed along with the
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
  -w, --workload string       Name of the service/job that access the storage.

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
                                    Must be either "v1" or "v2". (default "v2")

Others Flags
      --add-ingress-from string   The workload that needs access to an
                                  environment storage. Must be specified with
                                  "--name" and "--storage-type".
                                  Can be specified with "--engine".
```

## How can I use it? 
Create an S3 bucket named "my-bucket" attached to the "frontend" service.

```console
$ copilot storage init -n my-bucket -t S3 -w frontend -l workload
```

Create an environment S3 bucket named "my-bucket", fronted by the "api" service.
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
Copilot writes a Cloudformation template specifying the S3 bucket or DDB table to the `addons` dir. When you run `copilot svc deploy`, the CLI merges this template with all the other templates in the addons directory to create a nested stack associated with your service. This nested stack describes all the additional resources you've associated with that service and is deployed wherever your service is deployed. 

This means that after running
```console
$ copilot storage init -n bucket -t S3 -w fe
$ copilot svc deploy -n fe -e test
$ copilot svc deploy -n fe -e prod
```
there will be two buckets deployed, one in the "test" env and one in the "prod" env, accessible only to the "fe" service in its respective environment. 
