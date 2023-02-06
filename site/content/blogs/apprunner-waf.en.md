# Using WAF With AppRunner in Copilot

Posted On: Feb 08, 2023

**Siddharth Vohra**, Software Development Engineer, AWS App Runner

You can now associate your AWS WAF WebACLs with your App Runner service - all in Copilot!

AWS Web Application Firewall (WAF) helps you monitor requests that are forwarded to your web applications and allows you to control access to your content. To do this, you need to create a WAF Web Application Control List (ACL) which contains many rule options for your application. You can add multiple rules and create a Web ACL. Once you’ve created a Web ACL through AWS WAF Console or the AWS CLI, copy and save its ARN. To use the WAF ACL with your App Runner service managed by Copilot, follow the steps below.

 Step 1: If you don’t already have an App Runner service created/deployed, run `copilot init` to create and configure an App Runner service. (Do not deploy it just yet!)  

Step 2: Go to the service's directory (users/\<your username\>/copilot/\<your App Runner service name\>). If you don't have a folder called 'addons' in this directory already, create a new folder and name it 'addons'. Now, go to the AR-WAF Addon discussion (Hyperlink coming soon) and follow the steps.  Your folders would now look like this:  

  ```term
  .
  └── copilot
      └── <your App Runner service name>
          └── addons
              └── ar_waf_addon.yml 
              └── addons.parameters.yml
  ```

and `ar_waf-addon.yml` would look like  
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
  # your App Runner service
  Firewall:
    Metadata:
      'aws:copilot:description': 'Associating your App Runner service with your WAF WebACL'
    Type: AWS::WAFv2::WebACLAssociation
    Properties: 
      ResourceArn: !Sub ${ServiceARN}
      WebACLArn:  <paste your WAF Web ACL ARN here> #Paste your WAF Web ACLL ARN here
  ```

while `addons.parameters.yml` would look like:  
  ```yaml
  Parameters:
    ServiceARN: !Ref Service
  ```

Step 3: Open `ar_waf_addon.yml` and edit it to add your WebACL ARN where it is required - `Firewall.Properties.WebACLArn`. For example:   
  ```yaml
Resources:
  # Configuration of the WAF Web ACL you want to asscoiate with 
  # your App Runner service
  Firewall:
    Metadata:
      'aws:copilot:description': 'Associating your App Runner service with your WAF WebACL'
    Type: AWS::WAFv2::WebACLAssociation
    Properties: 
      ResourceArn: !Sub ${ServiceARN}
      WebACLArn: arn:aws:wafv2:us-east-2:637867102138:regional/webacl/mytestwebacl/3df32464-be9f-47ce-a12b-3a466c1c8913
  ```
 

Step 4: Now save the file and run `copilot svc deploy` to deploy your new App Runner service. Your App Runner service will now be deployed along with a WAF WebACL!  

Some considerations:  
-  A Web ACL can be linked to multiple services but one service can not be linked to more than one Web ACL
- If you already have an App Runner service deployed through Copilot, all you need to do is follow Steps 2-4 and you will be able to add a WAF Web ACL to your existing App Runner service.


