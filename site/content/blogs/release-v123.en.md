---
title: 'AWS Copilot v1.23: App Runner Private Services, Aurora Serverless v2 and more!'
twitter_title: 'AWS Copilot v1.23'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.23: App Runner Private Services, Aurora Serverless v2 and more!

Posted On: Nov 1, 2022

The AWS Copilot core team is announcing the Copilot v1.23 release.   
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has over 300 people online and nearly 2.5k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.23 brings several new features and improvements:

- **App Runner Private Services**: App Runner just launched support for private services, and you can create them by adding `http.private` to your Request-Driven Web Service manifest! [See detailed section](#app-runner-private-services).
- **Support Aurora Serverless v2 in `storage init`**: [See detailed section](#support-aurora-serverless-v2-in-storage-init).
- **Move misplaced `http` fields in environment manifest (backward-compatible!):** [See detailed section](#move-misplaced-http-fields-in-environment-manifest-backward-compatible).
- **Restrict container access to root file system to read-only:** [See manifest field](../docs/manifest/lb-web-service.en.md#storage-readonlyfs) ([#4062](https://github.com/aws/copilot-cli/pull/4062)).
- **Configure SSL policy for your ALB’s HTTPS listener:** [See manifest field](../docs/manifest/environment.en.md#http-public-sslpolicy) ([#4099](https://github.com/aws/copilot-cli/pull/4099)).
- **Restrict ingress to your ALB through source IPs**: [See manifest field](../docs/manifest/environment.en.md#http-public-ingress-source-ips) ([#4103](https://github.com/aws/copilot-cli/pull/4103)).


???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## App Runner Private Services
You can now create App Runner private services using Copilot. Simply update your Request-Driven Web Service manifest with:
```yaml
http:
  private: true
```
And deploy! Your service is now only reachable by other services in your Copilot environment.
Behind the scenes, Copilot creates a VPC Endpoint to App Runner that gets shared across all private services in your environment.
If you have an existing App Runner VPC Endpoint, you can import it by setting the following in your manifest:
```yaml
http:
  private:
    endpoint: vpce-12345
```
By default, your private service can only send traffic to the internet.
If you'd like to a send traffic to your environment, set [`network.vpc.placement: 'private'`](../docs/manifest/rd-web-service.en.md#network-vpc-placement) in your manifest.

## Support Aurora Serverless v2 in [`storage init`](../docs/commands/storage-init.en.md)
[Aurora Serverless v2 was made generally available earlier this year](https://aws.amazon.com/about-aws/whats-new/2022/04/amazon-aurora-serverless-v2/), 
and is now supported as a storage option in Copilot.

Previously, you could run 
```console
$ copilot storage init --storage-type Aurora
``` 
to generate an addon template for a v1 cluster. Now, **it will generate the template for v2 by default**. 
However, you can still use `copilot storage init --storage-type Aurora --serverless-version v1` to generate a v1 template.

For more, check out [the doc for `storage init`](../docs/commands/storage-init.en.md)!


## Move misplaced `http` fields in environment manifest (backward-compatible!)

In [Copilot v1.23.0](https://github.com/aws/copilot-cli/releases/tag/v1.23.0), we are fixing the hierarchy
under the `http` field in the environment manifest.

### What is getting fixed, and why?
Back in [Copilot v1.20.0](../blogs/release-v120.en.md), we released the environment manifest,
bringing all the benefits of infrastructure as code to environments. At the time, its `http` field hierarchy looked like:
```yaml
name: test
type: Environment

http:
  public:
    security_groups:
      ingress:         # [Flaw 1]
        restrict_to:   # [Flaw 2]
          cdn: true
  private:
    security_groups:
      ingress:         # [Flaw 1]
        from_vpc: true # [Flaw 2]
```
There are two flaws in this hierarchy design:

1. **Putting `ingress` under `security_groups` is ambiguous.** Each security group has its own ingress - it is unclear what
   the "ingress" of several security groups means. *(Here, it was meant to configure the ingress of
   the default security group that Copilot applies to an Application Load Balancer.)*

2. **`restrict_to` is redundant.** It should be clearly implied that the `ingress` under `http.public` is restrictive,
   and the `ingress` under `http.private` is permissive. The `"from"` in `from_vpc` also suffers from the same redundancy issue.

To illustrate - fixing them would give us an environment manifest that looks like:
```yaml
name: test
type: Environment

http:
  public:
    ingress:
      cdn: true
  private:
    ingress:
      vpc: true
```

### What do I need to do?

The short answer: nothing for now.

#### (Recommended) Adapt your manifest to the corrected hierarchy
While your existing manifest will keep working (we will get to this later), it is recommended that you update your manifest to the corrected hierarchy. 
Below are snippets detailing how to update each of the fields impacted:

???+ note "Adapt your environment manifest to the corrected hierarchy"

    === "CDN for public ALB"

        ```yaml
        # If you have
        http:
          public:
            security_groups:
              ingress:      
                restrict_to: 
                  cdn: true
        
        # Then change it to
        http:
          public:
            ingress:
              cdn: true
        ```

    === "VPC ingress for private ALB"
        ```yaml
        # If you have
        http:
          private:
            security_groups:
              ingress:      
                from_vpc: true
        
        # Then change it to
        http:
          private:
            ingress:
              vpc: true
        ```


#### Your existing environment manifest will keep working
It's okay if you don't adapt your environment manifest to the corrected hierarchy immediately. It will keep working - unless you modify your manifest so that it contains both `http.public.security_groups.ingress` (the flawed version) 
and `http.public.ingress` (the corrected version).

For example, say before the release of v1.23.0, your manifest looked like:
```yaml
# Flawed hierarchy but will keep working.
http:
  public:
    security_groups:
      ingress:      
        restrict_to: 
          cdn: true
```
The same manifest will keep working after v1.23.0.

However, if at some point you modify the manifest to:
```yaml
# Error! Both flawed hierarchy and corrected hierarchy are present.
http:
  public:
    security_groups:
      ingress:      
        restrict_to: 
          cdn: true
    ingress:
      source_ips:
        - 10.0.0.0/24
        - 10.0.1.0/24
```
Copilot will detect that both  `http.public.security_groups.ingress` (the flawed version) and
`http.public.ingress` exist in the manifest. It will error out, along with a friendly suggestion to update your manifest
so that only `http.public.ingress`, the corrected version is present:
```yaml
# Same configuration but written in the corrected hierarchy.
http:
  public:
    ingress:
        cdn: true
        source_ips:
            - 10.0.0.0/24
            - 10.0.1.0/24
```
## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.23.0)
