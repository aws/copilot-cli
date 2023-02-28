# Using WAF With App Runner in Copilot

Posted On: Feb 23, 2023

**Siddharth Vohra**, Software Development Engineer, AWS App Runner

You can now associate your AWS Web Application Firewall (WAF) Web access control lists (web ACLs) with your App Runner service - all in Copilot!

[AWS Web Application Firewall (WAF)](https://docs.aws.amazon.com/waf/latest/developerguide/waf-chapter.html) helps you monitor the HTTP(S) requests that are forwarded to your web applications,
and allows you to control access to your content. 
You can protect your application by blocking or allowing web requests based on criteria that you specify, 
such as the IP addresses that requests originate from or the values of query strings.

Today, AWS announced [WAF support for AWS App Runner](https://aws.amazon.com/about-aws/whats-new/2023/02/aws-app-runner-web-application-firewall-enhanced-security/). 
This means that you can now protect your [Request-Driven Web Services](../docs/concepts/services.en.md#request-driven-web-service) with AWS WAF. 
This blog post will show you how to easily enable the protection using AWS Copilot.



!!!info
    We posted these steps in our [GitHub "Show and tell" discussion section](https://github.com/aws/copilot-cli/discussions/4542) as well! If you have any questions, feedbacks or requests that are related to App Runner's WAF support, feel free to drop a comment there!


### Prerequisite
To proceed, you need to bring your own WAF Web Application Control List (ACL). 
If you don't have one already, you need to first create a WAF ACL with rule options 
for your application (See [here](https://docs.aws.amazon.com/waf/latest/developerguide/web-acl-creating.html) for more on creating your own Web ACL).

Once your Web ACL is ready, note down its ARN. 
To use the WAF Web ACL with your Request-Driven Web Service, follow the steps below.  

### Step 1 (Optional): Create a Request-Driven Web Service
If you don’t already have a Request-Driven Web Service, 
you can run the following command to create and configure an App Runner service.
```console
copilot init \
  --svc-type "Request-Driven Web Service" \
  --name "waf-example" \
  --dockerfile path/to/Dockerfile
```
Alternatively, you can simply run `copilot svc init` without any flags. Copilot will prompt for information and
guide you through the process.

### Step 2 (Optional): Create an `addons/` folder for your service

Stay in the workspace where your Request-Driven Web Service is. If you followed step 1, then your working directory's 
structure may look like:
```term
.
└── copilot/
  └── waf-example/ # The name of your Request-Driven Web Service. Not necessarily "waf-example".
      └── manifest.yml
```

If you don't have `addons/` folder under `./copilot/<name of your Request-Driven Web Service>` yet, create one.
Now your workspace may look like:
```term
.
└── copilot/
  └── waf-example/ # The name of your Request-Driven Web Service. Not necessarily "waf-example".
      ├── manifest.yml
      └── addons/
```

### Step 3: Associate Web ACL with your Request-Driven Web Service using addon.

In the addons folder, create two new files: `waf.yml` and `addons.parameters.yml`. Your folders would now look like this:  

  ```term
  .
  └── copilot
      └── waf-example/ # The name of your Request-Driven Web Service. Not necessarily "waf-example".
          ├── manifest.yml
          └── addons/
              ├── waf.yml 
              └── addons.parameters.yml
  ```

Copy and paste the following content to the respective files:  

=== "waf.yml"
    ```yaml
    #Addon template to add WAF configuration to your App Runner service.
    
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
      ServiceARN:
        Type: String
        Default: ""
        Description: The ARN of the service being deployed.
    
    Resources:
      # Configuration of the WAF Web ACL you want to asscoiate with 
      # your App Runner service.
      Firewall:
        Metadata:
          'aws:copilot:description': 'Associating your App Runner service with your WAF WebACL'
        Type: AWS::WAFv2::WebACLAssociation
        Properties: 
          ResourceArn: !Sub ${ServiceARN}
          WebACLArn:  <paste your WAF Web ACL ARN here> # Paste your WAF Web ACL ARN here.
    ```

=== "addons.parameters.yml"  
      ```yaml
      Parameters:
        ServiceARN: !Ref Service
      ```


### Step 4: Populate your Web ACL ARN in `waf.yml`

Open `waf.yml` and replace `<paste your WAF Web ACL ARN here>` with the ARN of your Web ACL resource. For example:   
```yaml
Resources:
  # Configuration of the WAF Web ACL you want to associate with 
  # your App Runner service
  Firewall:
    Metadata:
      'aws:copilot:description': 'Associating your App Runner service with your WAF WebACL'
    Type: AWS::WAFv2::WebACLAssociation
    Properties: 
      ResourceArn: !Sub ${ServiceARN}
      WebACLArn: arn:aws:wafv2:us-east-2:123456789138:regional/webacl/mytestwebacl/3df43564-be9f-47ce-a12b-3a577d2d8913
```
 

### Step 5: Deploy your service 
Finally, run `copilot svc deploy`! Your Request-Driven Web service will now be deployed and be associated with a WAF Web ACL!  

???+ note "Some considerations"
    - A Web ACL can be linked to multiple services but one service can not be linked to more than one Web ACL
    - If you already have an App Runner service deployed through Copilot, all you need to do is follow Steps 2-5, and you will be able to add a WAF Web ACL to the existing service.
