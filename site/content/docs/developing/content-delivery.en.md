# Global Content Delivery

Copilot supports a Content Delivery Network through AWS CloudFront. This resource is managed by Copilot at the environment level, allowing users to leverage CloudFront through the [environment manifest](../manifest/environment.en.md).

## CloudFront infrastructure with Copilot

When Copilot creates a [CloudFront distribution](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-overview.html), it creates the distribution to be the new entry point to the application instead of the Application Load Balancer. This allows CloudFront to route your traffic to the load balancer faster via the edge locations deployed around the globe.

## How do I use CloudFront with my existing application?

As of Copilot v1.20, your environment will be created with a manifest file, similar to a workload (service or job) manifest. In this manifest, you can specify the value `cdn: true` and then run `copilot env deploy` to enable a basic CloudFront distribution.

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

When using HTTPS with CloudFront, specify your certificates in the `http.certificates` field of the environment manifest, just as you would for a Load Balancer. The new field `cdn.certificate` similarly enables HTTPS traffic with CloudFront. Note that this certificate may only be imported in the `us-east-1` region.

!!! info
    Importing a certificate for CloudFront adds an extra permission to your Environment Manager Role, allowing Copilot to use the `DescribeCertificate` [API call](https://docs.aws.amazon.com/acm/latest/APIReference/API_DescribeCertificate.html).

You can also let Copilot manage the certificates for you by specifying `--domain` when you create your application. When doing this, you must specify `http.alias` for all your services deployed in the CloudFront-enabled environment.

With both of these setups, Copilot will provision CloudFront to use the [SSL/TLS Certificate](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-https-alternate-domain-names.html), which allows it to validate the viewer certificate, enabling an HTTPS connection.