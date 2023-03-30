# Domain

## Load Balanced Web Service
In Copilot, there are two ways to use custom domains for your Load Balanced Web Service:

1. Use `--domain` when creating an application to associate a Route 53 domain in the same account.
2. Use the [`http.[public/private].certificates`](../manifest/environment.en.md#http-public-certificates) field in an environment manifest to import your validated ACM certificates into the environment.

!!!attention
    Today, a Route 53 domain name can only be associated when running `copilot app init`.  
    If you'd like to update your application with a domain ([#3045](https://github.com/aws/copilot-cli/issues/3045)),
    you'll need to run `copilot app delete` to remove the old one before creating a new one with `--domain` to associate a new domain.

### Use app-associated root domain

As mentioned in the [Application Guide](../concepts/applications.en.md#additional-app-configurations), you can configure the domain name of your app when running `copilot app init --domain`.

**Default domain name for your service**

After deploying your [Load Balanced Web Services](../concepts/services.en.md#load-balanced-web-service), by default you can access them publicly via

```
${SvcName}.${EnvName}.${AppName}.${DomainName}
```

if you are using an Application Load Balancer, or

```
${SvcName}-nlb.${EnvName}.${AppName}.${DomainName}
```

if you are using a Network Load Balancer.

For example, `https://kudo.test.coolapp.example.aws` or `kudo-nlb.test.coolapp.example.aws:443`.

#### Customized domain alias

If you don't like the default domain name Copilot assigns to your service, you can set a custom [alias](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html) for your service by editing your [manifest's](../manifest/lb-web-service.en.md#http-alias) `alias` section.
The following snippet sets an alias to your service.

``` yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
  alias: example.aws
```

Similarly, if your service is using a Network Load Balancer, you can specify:
```yaml
nlb:
  port: 443/tls
  alias: example-v1.aws
```

However, since we [delegate responsibility for the subdomain to Route 53](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/CreatingNewSubdomain.html#UpdateDNSParentDomain), the alias you specify must follow one of the following Copilot-enabled patterns:

- `{domain}`, such as `example.aws`
- `{subdomain}.{domain}`, such as `v1.example.aws`
- `{appName}.{domain}`, such as `coolapp.example.aws`
- `{subdomain}.{appName}.{domain}`, such as `v1.coolapp.example.aws`
- `{envName}.{appName}.{domain}`, such as `test.coolapp.example.aws`
- `{subdomain}.{envName}.{appName}.{domain}`, such as `v1.test.coolapp.example.aws`

#### What happens under the hood?

Under the hood, Copilot

* creates a hosted zone in your app account for the new app subdomain `${AppName}.${DomainName}`
* creates another hosted zone in your env account for the new env subdomain `${EnvName}.${AppName}.${DomainName}`
* creates and validates an ACM certificate for the env subdomain
* associates the certificate with:
    - Your HTTPS listener and redirects HTTP traffic to HTTPS, if the alias is used for the Application Load Balancer (`http.alias`)
    - Your network load balancer's TLS listener, if the alias is for `nlb.alias` and TLS termination is enabled.
* creates an optional A record for your alias

#### What does it look like?

<iframe width="560" height="315" src="https://www.youtube.com/embed/Oyr-n59mVjI" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

### Use domain in your existing validated certificates
If you'd like more granular control over the generated ACM certificate, or if the [default `alias` options](#customized-domain-alias) aren't flexible enough,
you can import validated ACM certificates that include the alias to your environment.
In the [environment manifest](../manifest/environment.en.md), specify `http.[public/private].certificates`:

```yaml
type: Environment
http:
  public:
    certificates:
      - arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012
```

Then, in your service's manifest, you can:

- Specify the ID of the [`hosted zone`](../manifest/lb-web-service.en.md#http-hosted-zone) into which Copilot should insert the A record:
``` yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
  alias: example.aws
  hosted_zone: Z0873220N255IR3MTNR4
```
- Alternatively, deploy the service without the `hosted_zone` field, then manually add the DNS name of the Application Load Balancer (ALB) created in that environment as an A record where your alias domain is hosted.

We have [an example](../../blogs/release-v118.en.md#certificate-import) of Option 2 in our blog posts.

## Request-Driven Web Service
You can also add a [custom domain](https://docs.aws.amazon.com/apprunner/latest/dg/manage-custom-domains.html) for your request-driven web service.
Similar to Load Balanced Web Service, you can do so by modifying the [`alias`](../manifest/rd-web-service.en.md#http-alias) field in your manifest:
```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
  alias: web.example.aws
```

Likewise, your application should have been associated with the domain (e.g. `example.aws`) in order for your Request-Driven Web Service to use it.

!!!info
    For now, we support only one-level subdomains such as `web.example.aws`.

    Environment-level domains (e.g. `web.${envName}.${appName}.example.aws`), application-level domains (e.g. `web.${appName}.example.aws`),
    or root domains (i.e. `example.aws`) are not supported yet. This also means that your subdomain shouldn't collide with your application name.

Under the hood, Copilot:

* associates the domain with your app runner service
* creates the domain record as well as the validation records in your root domain's hosted zone
