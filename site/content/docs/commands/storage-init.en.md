# storage init
```console
$ copilot storage init
```
## What does it do?
`copilot storage init` creates a new storage resource attached to one of your workloads, accessible from inside your service container via a friendly environment variable. You can specify either *S3*, *DynamoDB* or *Aurora* as the resource type.

After running this command, the CLI creates an `addons` subdirectory inside your `copilot/service` directory if it does not exist. When you run `copilot svc deploy`, your newly initialized storage resource is created in the environment you're deploying to. By default, only the service you specify during `storage init` will have access to that storage resource.

## What are the flags?
```
Required Flags
  -n, --name string           Name of the storage resource to create.
  -t, --storage-type string   Type of storage to add. Must be one of:
                              "DynamoDB", "S3", "Aurora".
  -w, --workload string       Name of the service or job to associate with storage.

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
      --engine string           The database engine used in the cluster.
                                Must be either "MySQL" or "PostgreSQL".
      --parameter-group string  Optional. The name of the parameter group to associate with the cluster.
      --initial-db string       The initial database to create in the cluster.
```

## How can I use it? 
Create an S3 bucket named "my-bucket" attached to the "frontend" service.

```console
$ copilot storage init -n my-bucket -t S3 -w frontend
```
Create a basic DynamoDB table named "my-table" attached to the "frontend" service with a sort key specified.

```console
$ copilot storage init -n my-table -t DynamoDB -w frontend --partition-key Email:S --sort-key UserId:N --no-lsi
```

Create a DynamoDB table with multiple alternate sort keys.

```console
$ copilot storage init \
  -n my-table -t DynamoDB -w frontend \
  --partition-key Email:S \
  --sort-key UserId:N \
  --lsi Points:N \
  --lsi Goodness:N
```

Create an RDS Aurora Serverless cluster using PostgreSQL as the database engine.
```console
$ copilot storage init \
  -n my-cluster -t Aurora -w frontend --engine PostgreSQL
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
