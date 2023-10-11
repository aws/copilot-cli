---
title: 'AWS Copilot v1.31: NLB enhancements, better task failure logs, `copilot deploy` enhancements'
twitter_title: 'AWS Copilot v1.31'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# 'AWS Copilot v1.31: NLB enhancements, better task failure logs, `copilot deploy` enhancements

Posted On: October 5, 2023

The AWS Copilot core team is announcing the Copilot v1.31 release.

Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) has over 500 people participating, and our GitHub repository has over 3k stars on [GitHub](http://github.com/aws/copilot-cli/) 🚀.
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.31 brings big enhancements to help you develop more flexibly and efficiently:

- **NLB enhancements**: You can now add security groups to Copilot-managed [network load balancers](../docs/manifest/lb-web-service.en.md#nlb). NLBs also support the UDP protocol. [See detailed section](#nlb-enhancements)
- **Better task failure logs**: Copilot will show more descriptive information during deployments when tasks fail, allowing better troubleshooting.
- **`copilot deploy` enhancements**: You can now deploy multiple workloads at once, or deploy all local workloads with `--all`.
- **Importing an ACM certificate for your Static Site**: You can now bring your own ACM certificate for the Static Site service. [See detailed section](#importing-an-acm-certificate-for-your-static-site)

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision Infrastructure as Code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## NLB enhancements

Copilot brings UDP traffic support with an update to your Network Load Balancer! The protocol your NLB uses is specified by the [nlb.port](https://aws.github.io/copilot-cli/docs/manifest/lb-web-service/#nlb-port) field.
```
nlb:
  port: 8080/udp
```

!!!info 
    NLB Security Group is a new AWS feature that lets you filter public traffic to your NLB, enhancing the security of your application. For more information, read this [AWS blogpost](https://aws.amazon.com/blogs/containers/network-load-balancers-now-support-security-groups/). For Copilot to use this feature, your `NetworkLoadBalancer` and `TargetGroup` resources need to be recreated. With v1.31 this will only happen if you specify `udp` protocol. With v1.33 however, Copilot will make this change for all users. This means that if you don't use DNS aliases, then the NLB's domain name will change, and if you do use DNS alias, then the alias will start pointing to the new NLB that is enhanced with a security group.

## `copilot deploy` enhancements
`copilot deploy` now supports deploying multiple workloads with one command. You can specify multiple workloads with the
`--name` flag, use the new `--all` flag in conjunction with `--init-wkld` to initialize and deploy all local workloads,
and you can now provide a "deployment order" tag when specifying service names. 

For example, if you have cloned a new repository which includes multiple workloads, you can initialize the environment and 
all services with one command.
```console
copilot deploy --init-env --deploy-env -e dev --all --init-wkld
```

If you have a service which must be deployed before another--for example, there is worker service which subscribes to a topic exposed
by a different service in the workspace--you can specify names and orders with `--all`.
```console
copilot deploy --all -n fe/1 -n worker/2
```
This will deploy `fe`, then `worker`, then the remaining services or jobs in the workspace.

## Better task failure logs

Before Copilot v1.31, if you wanted to find out why your ECS tasks stopped, you'd have to navigate to AWS Console -> ECS -> Service -> Stopped Tasks -> Stopped Reason.

With this enhancement, `copilot [noun] deploy` will now display the ECS task stopped reasons within your CloudFormation deployment progress tracker. Copilot will show the two most recent task failures during deployments of your Load Balanced Web Service, Backend Service and Worker Services.

```console
  - An ECS service to run and maintain your tasks in the environment cluster
    Deployments                                                                                                              
               Revision  Rollout        Desired  Running  Failed  Pending                                                            
      PRIMARY  11        [in progress]  1        0        1       0                                                                  
      ACTIVE   8         [completed]    1        1        0       0                                                                  
    Latest 2 stopped tasks                                                                                                   
      TaskId    CurrentStatus   DesiredStatus                                                                                        
      6b1d6e32  DEPROVISIONING  STOPPED                                                                                              
      9802d212  STOPPED         STOPPED                                                                                              
                                                                                                                                     
    ✘ Latest 2 tasks stopped reason                                                                                 
      - [6b1d6e32,9802d212]: Essential container in task exited                                                                      
                                                                                                                                     
    Troubleshoot task stopped reason                                                                                         
      1. You can run `copilot svc logs --previous` to see the logs of the last stopped task.                                
      2. You can visit this article: https://repost.aws/knowledge-center/ecs-task-stopped.          
```


## Importing an ACM certificate for your Static Site

Copilot now introduces a new field `http.certificate` in the [Static Site manifest](../docs/manifest/static-site.en.md). You can specify with the ARN of any validated ACM certificate in `us-east-1` as below:

```yaml
http:
  alias: example.com
  certificate: "arn:aws:acm:us-east-1:1234567890:certificate/e5a6e114-b022-45b1-9339-38fbfd6db3e2"
```

Note that `example.com` must be the domain name or any subject alternative name of the certificate you are bringing in, and we'll use the imported certificate for your HTTPS traffic instead of creating a Copilot-managed one.
