---
title: 'AWS Copilot v1.21: CloudFront is here!'
twitter_title: 'AWS Copilot v1.21'
image: 'https://user-images.githubusercontent.com/10566468/184949047-f4f173ae-0b29-47fd-8c0b-14a212029587.png'
image_alt: 'v1.21 Environment Manifest and Lambda Addon'
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.21: CloudFront is here!

Posted On: Aug 17, 2022

The AWS Copilot core team is announcing the Copilot v1.21 release.  
Special thanks to [@dave-moser](https://github.com/dave-moser), [@dclark](https://github.com/dclark), and [@apopa57](https://github.com/apopa57) who contributed to this release.
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has over 300 people online and over 2.4k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.21 brings several new features and improvements:

- **Integrate CloudFront with Application Load Balancer**: You can now deploy CloudFront in front of your Load Balanced Web Services! [See detailed section](#cloudfront-integration) to learn more.
- **Configure environment security group**: Configure your environment security group rules through environment manifest. [See detailed section](#configure-environment-security-group).
- **ELB access log support**: Enable elastic load balancing access logs for your Load Balanced Web Service. [See detailed section](#elb-access-logs-support).
- **`job logs` improvements**: You can now follow logs and view state machine execution logs for your jobs. [See detailed Section](#job-logs)
- **Package addon CloudFormation templates before deployments**: Copilot will now package addon templates on `copilot svc deploy`. This means Copilot can now deploy AWS Lambda functions alongside your containerized services! Read more about how to get started in [Copilot's documentation](../docs/developing/addons/package.en.md).

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## CloudFront Integration

One of our first major additions to the Copilot environment manifest! CloudFront is an AWS Content Delivery Network (CDN) which helps people deploy their applications across the globe, and now you can enable a distribution by simply setting `cdn: true` in your environment manifest and running `copilot env deploy`.

### Currently supported features
- A distribution deployed in front of your public Application Load Balancer (ALB)
- ALB ingress restricted to the CloudFront distribution to protect from DDoS attack
- HTTPS traffic through an imported certificate, or a Copilot-managed certificate

### CloudFront with HTTPS
Copilot makes this process easy! If you have an application initialized with a `--domain` specified during `app init`, the required certificate will be created for you and no additional actions are required. 

If you import your own certificates for your hosted zone, we'll walk you through importing the correct certificate for CloudFront.

!!!info
    CloudFront requires certificates to be in the `us-east-1` region. When importing a certificate, make sure to create your certificate in this region.

First, create a certificate in the `us-east-1` region for your application with [AWS Certificate Manager](https://aws.amazon.com/certificate-manager/). You must add each domain associated with your application to this certificate. Once you've validated the certificate, you can add a field to your environment manifest to import the certificate for CloudFront:
```yaml
cdn:
  certificate: arn:aws:acm:us-east-1:${AWS_ACCOUNT_ID}:certificate/13245665-h74x-4ore-jdnz-avs87dl11jd
```
Run `copilot env deploy`, then you can create an A-record in [Route 53](https://aws.amazon.com/route53/) which points to the CloudFront distribution created by Copilot. Just select to point the record to an `Alias` in the console, then select to route traffic to a CloudFront distribution resource type, and enter the CloudFront DNS from the deployed distribution. 

### Restricting traffic to CloudFront
To restrict public traffic to come through the CloudFront distribution, there's a new field in `http` for your public load balancer:
```yaml
http:
  public:
    security_groups:
      ingress:
        restrict_to:
          cdn: true
```
Specifying this will modify the Load Balancer's security group to only accept traffic from CloudFront.

## Configure Environment Security Group
You can now configure your environment security group rules through environment manifest.   
Sample security group rules template inside environment manifest is given below.
```yaml
network:
  vpc:
    security_group:
      ingress:
        - ip_protocol: tcp
          ports: 80
          cidr: 0.0.0.0/0
      egress:
        - ip_protocol: tcp
          ports: 0-65535
          cidr: 0.0.0.0/0
```
For the complete specification, see the [environment manifest](../docs/manifest/environment.en.md#network-vpc-security-group).

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
You can also view the bucket name with `copilot env show --resources` command.

2. You can also bring in your own bucket and prefix. Copilot will use those bucket details to enable access logs.
   You can do that by specifying the following configuration in your environment manifest.
```yaml
name: qa
type: Environment

http:
 public:
   access_logs:
     bucket_name: my-bucket
     prefix: my-prefix
```
When importing your own bucket, you need to make sure that the bucket exists and has the required [bucket policy](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/enable-access-logs.html#attach-bucket-policy) for the load balancer to
write access logs to it.

## `job logs`
At long last, you can now view and follow logs for executions of your scheduled jobs. 
You can choose how many invocations of the job to view, filter logs by specific task IDs, and choose whether to view state machine execution logs. 
For example, you can view logs from the last invocation of the job and all the state machine execution data:
```console
$ copilot job logs --include-state-machine
Which application does your job belong to? [Use arrows to move, type to filter, ? for more help]
> app1
  app2
Which job's logs would you like to show? [Use arrows to move, type to filter, ? for more help]
> emailer (test)
  emailer (prod)
Application: app1
Job: emailer
states/app1-test-emailer {"id":"1","type":"ExecutionStarted","details": ...
states/app1-test-emailer {"id":"2","type":"TaskStateEntered","details": ...
states/app1-test-emailer {"id":"3","type":"TaskScheduled","details": ...
states/app1-test-emailer {"id":"4","type":"TaskStarted","details": ...
states/app1-test-emailer {"id":"5","type":"TaskSubmitted","details": ...
copilot/emailer/d476069 Gathered recipients
copilot/emailer/d476069 Prepared email body 
copilot/emailer/d476069 Attached headers
copilot/emailer/d476069 Sent all emails
states/app1-test-emailer {"id":"6","type":"TaskSucceeded","details": ...
states/app1-test-emailer {"id":"7","type":"TaskStateExited","details": ...
states/app1-test-emailer {"id":"8","type":"ExecutionSucceeded","details": ...

```
or follow the logs of a task you've just invoked with [`copilot job run`](../docs/commands/job-run.en.md):
```console
$ copilot job run -n emailer && copilot job logs -n emailer --follow
```
## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.21.0)
