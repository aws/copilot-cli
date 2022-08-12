# AWS Copilot v1.21: CloudFront is coming!

Posted On: Aug 15, 2022

The AWS Copilot core team is announcing the Copilot v1.21 release.  
Special thanks to [@dave-moser](https://github.com/dave-moser) who contributed to this release.
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has over 300 people online and over 2.3k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.21 brings several new features and improvements:

- **Integrate CloudFront with Application Load Balancer**:
- **Configure environment security group**:
- **Check out job logs**:
- **Package addons CloudFormation templates before deployments**:
- **ELB access log support**:

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## CloudFront Integration

## Configure Environment Security Group
You can now configure your environment security group rules through environment manifest.   
Sample security group rules template inside environment manifest is given below.
```yaml
network:
  vpc:
    security_group:
      ingress:
        - ip_protocol: tcp
          ports: 0-65535
          cidr: 0.0.0.0/0
      egress:
        - ip_protocol: tcp
          ports: 80
          cidr: 0.0.0.0/0
```
## `job logs`

## Package Addons CloudFormation Templates

## ELB Access Logs Support
You can now enable Elastic Load Balancing access logs that capture detailed information about requests sent to your load balancer.
There are a few ways to enable access logs: 

1. You can specify `access_logs: true` in your environment manifest as shown below and Copilot will create an S3 bucket where the Public Load Balancer will store access logs.
```yaml
name: qa
type: Environment

http:
  public:
    access_logs: true 
```

2. You can also bring in your own bucket and prefix. Copilot will use those bucket details to enable access logs. 
You can do that by specifying the following configuration in your environment manifest.
```yaml
name: qa
type: Environment

http:
 public:
   access_logs:
     bucket_name: my-bucket
     bucket_prefix: my-prefix
```
When importing your own bucket, you need to make sure that the bucket exists and has the required [bucket policy](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/enable-access-logs.html#attach-bucket-policy) for the load balancer to
write access logs to it.

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.21.0)
