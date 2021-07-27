# Domain

## Load Balanced Web Service
As mentioned in the [Application Guide](../concepts/applications.en.md#additional-app-configurations), you can configure the domain name of your app when running `copilot app init`. After deploying your [Load Balanced Web Services](../concepts/services.en.md#load-balanced-web-service), you should be able to access them publicly via

```
${SvcName}.${EnvName}.${AppName}.${DomainName}
```

For example:

```
https://kudo.test.coolapp.example.aws
```

Currently, you can only use aliases under the domain you specified when creating the application. Since we [delegate responsibility for the subdomain to Route 53](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/CreatingNewSubdomain.html#UpdateDNSParentDomain), the alias you specify must be in either one of these three hosted zone:

- root: `${DomainName}`
- app: `${AppName}.${DomainName}`
- env: `${EnvName}.${AppName}.${DomainName}`

We'll make this feature more powerful in the future by allowing you to import certificates and use any aliases!

!!!info
    Both root and app hosted zone are in your app account, while the env hosted zones are in your env accounts.

## How do I configure an alias for my service?
If you don't like the default domain name Copilot assigns to your service, setting an [alias](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html) for your service is also very easy. You can add it directly to your [manifest's](../manifest/overview.en.md) `alias` section. The following snippet will set an alias to your service.

``` yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
  alias: example.aws
```

!!!info
    Using this feature requires your app version to be at least `v1.0.0`. You will be prompted to run [`app upgrade`](../commands/app-upgrade.en.md) first if your app version does not meet the requirement.

## What happens under the hood?
Under the hood, Copilot

* creates a hosted zone in your app account for the new app subdomain `${AppName}.${DomainName}`
* creates another hosted zone in your env account for the new env subdomain `${EnvName}.${AppName}.${DomainName}`
* creates and validates an ACM certificate for the env subdomain
* associates the certificate with your HTTPS listener and redirects HTTP traffic to HTTPS
* creates an optional A record for your alias

## What does it look like?

<iframe width="560" height="315" src="https://www.youtube.com/embed/Oyr-n59mVjI" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

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

Under the hood, Copilot:
* associates the domain with your app runner service
* creates the domain record as well as the validation records in your root domain's hosted zone
