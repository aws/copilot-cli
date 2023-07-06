# Global Content Delivery

Copilot supports a Content Delivery Network through Amazon CloudFront. This resource is managed by Copilot at the environment level, allowing users to leverage CloudFront through the [environment manifest](../manifest/environment.en.md).

## CloudFront infrastructure with Copilot

When Copilot creates a [CloudFront distribution](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-overview.html), it creates the distribution to be the new entry point to the application instead of the Application Load Balancer. This allows CloudFront to route your traffic to the load balancer faster via the edge locations deployed around the globe.

## How do I use CloudFront with my existing application?

Starting with Copilot v1.20, `copilot env init` creates an environment manifest file. In this manifest, you can specify the value `cdn: true` and then run `copilot env deploy` to enable a basic CloudFront distribution.

???+ note "Sample CloudFront distribution manifest setups"

    === "Basic"

        ```yaml
        cdn: true

        http:
          public:
            security_groups:
              ingress:
                restrict_to:
                  cdn: true
        ```

    === "Imported Certificates"

        ```yaml
        cdn:
          certificate: arn:aws:acm:us-east-1:${AWS_ACCOUNT_ID}:certificate/13245665-h74x-4ore-jdnz-avs87dl11jd

        http:
          certificates:
            - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-bldz-0an1-afki-p7ll1myafd
            - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/56654321-cv8f-adf3-j7gd-adf876af95
        ```

## How do I enable HTTPS traffic with CloudFront?

When using HTTPS with CloudFront, specify your certificates in the `cdn.certificate` field of the environment manifest, just as you would in the `http.certificates` field for a Load Balancer. Unlike for a Load Balancer, you can import only one certificate. Because of this, we recommend that you create a new certificate (in the `us-east-1` region) with CNAME records to validate each alias that your services use in that environment.

!!! info
    CloudFront only supports certificates imported in the `us-east-1` region.

!!! info
    Importing a certificate for CloudFront adds an extra permission to your Environment Manager Role, allowing Copilot to use the `DescribeCertificate` [API call](https://docs.aws.amazon.com/acm/latest/APIReference/API_DescribeCertificate.html).

You can also let Copilot manage the certificates for you by specifying `--domain` when you create your application. When doing this, you must specify `http.alias` for all your services deployed in the CloudFront-enabled environment.

With both of these setups, Copilot will provision CloudFront to use the [SSL/TLS Certificate](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-https-alternate-domain-names.html), which allows it to validate the viewer certificate, enabling an HTTPS connection.

## What is ingress restriction?

You can restrict incoming traffic to come from a certain source. For CloudFront, Copilot uses an [AWS managed prefix list](https://docs.aws.amazon.com/vpc/latest/userguide/working-with-aws-managed-prefix-lists.html) to restrict allowed traffic to a set of CIDR IP addresses associated with CloudFront edge locations. When you specify `restrict_to.cdn: true`, your Public Load Balancer is no longer publicly accessible, and can only be accessed through the CloudFront distribution, guarding against security threats to your services.

## How do I use CloudFront to terminate TLS?

!!! attention
    1. Disable [HTTP to HTTPS redirection](../../manifest/lb-web-service/#http-redirect-to-https) for your Load Balanced Web Services.
    2. Run `svc deploy` individually to redeploy all the Load Balanced Web Services before enabling CloudFront TLS termination.
    3. Once all your Load Balanced Web Services no longer redirect HTTP to HTTPS, you can safely enable CloudFront TLS termination in the env manifest and run `env deploy`.


You can optionally use CloudFront for TLS termination by configuring the env manifest as

```yaml
cdn:
  terminate_tls: true
```

And traffic from `CloudFront → Application Load Balancer (ALB) → ECS` will be HTTP only. This brings the benefit of terminating TLS at a geographically closer endpoint to the end user for faster TLS handshakes.

## How do I use CloudFront with an S3 bucket?
You can optionally have CloudFront work with an Amazon S3 bucket for faster static content delivery by configuring `cdn.static_assets` in the env manifest.

### Use an existing S3 bucket

!!! attention
    For security concerns, we suggest that you use a **private** S3 bucket so that all public access is blocked by default.

The env manifest example below illustrates how to use an existing S3 bucket for CloudFront:

???+ note "Sample env manifest setup for using an existing S3 bucket with CloudFront"
    ```yaml
    cdn:
      static_assets:
        location: cf-s3-ecs-demo-bucket.s3.us-west-2.amazonaws.com
        alias: example.com
        path: static/*
    ```

Note that `static_assets.location` is the DNS domain name of the S3 bucket (for example, `EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com`). If you are not using an alias of the [app-associated root domain](../domain/#use-app-associated-root-domain), remember to create an A-record for your alias pointing to the CloudFront domain name.

After getting the environment deployed using the env manifest, you need to update the bucket policy of your S3 bucket (if it is a private one), so that CloudFront can access it.

???+ note "Example S3 bucket policy that grants read-only access to CloudFront"
    ```json
    {
        "Version": "2012-10-17",
        "Statement": {
            "Sid": "AllowCloudFrontServicePrincipalReadOnly",
            "Effect": "Allow",
            "Principal": {
                "Service": "cloudfront.amazonaws.com"
            },
            "Action": "s3:GetObject",
            "Resource": "arn:aws:s3:::EXAMPLE-BUCKET/*",
            "Condition": {
                "StringEquals": {
                    "AWS:SourceArn": "arn:aws:cloudfront::111122223333:distribution/EDFDVBD6EXAMPLE"
                }
            }
        }
    }
    ```

## How do I use CloudFront to serve a static website?
Use the [copilot init](../commands/init.en.md) or [copilot svc init](../commands/svc-init.en.md) command to create a Static Site [workload](../concepts/services.en.md). After you select files to upload, Copilot will provision a separate, dedicated CloudFront distribution, as well as an S3 bucket with your assets. With each redeployment, Copilot will invalidate the existing cache for more dynamic development.