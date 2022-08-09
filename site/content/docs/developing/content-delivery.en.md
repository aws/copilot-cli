# Global Content Delivery

Copilot supports a Content Delivery Network through AWS CloudFront. This resource is managed by Copilot at the environment level, allowing users to leverage CloudFront through the [environment manifest](../manifest/environment.en.md).

## CloudFront infrastructure with Copilot

When Copilot creates a [CloudFront distribution](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-overview.html), it creates the distribution to be the new entry point to the application instead of the Application Load Balancer. This allows CloudFront to route your traffic to the load balancer faster via the edge locations deployed around the globe.

## How do I use CloudFront with my existing application?

As of Copilot v1.20 your environment will be created with a manifest file; just like a service manifest. In this manifest you can specify the value `cdn: true` and then run `copilot env deploy` to enable a basic CloudFront distribution.

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
          certificate: cert

        http:
          certificates:
            - cert1
            - cert2
        ```

## How do I enable HTTPS traffic with CloudFront?

When using HTTPS, CloudFront uses the same paradigm for certificates as specifying in the `http.certificates` field of the environment manifest for a Load Balancer. This means that there's a new field: `cdn.certificate` which is used to enable HTTPS traffic with CloudFront. However, this certificate is constrained to be imported in the region `us-east-1`.

!!! info
    Importing a certificate for CloudFront adds an extra permission to your Environment Manager Role which allows Copilot to use `DescribeCertificate`.

You can also let Copilot manage the certificates for you by specifying `--domain` when you create your application. When doing this, we require you to specify `http.alias` for all your services deployed in the environment with CloudFront enabled.

With both of these setups, Copilot will then provision CloudFront to use the [SSL/TLS Certificate](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-https-alternate-domain-names.html) which allows it to validate the viewer certificate, enabling an HTTPS connection.