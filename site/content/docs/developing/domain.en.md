# Domain

As mentioned in the [Application Guide](../concepts/applications.en.md#additional-app-configurations), you can configure the domain name of your app when running `copilot app init`. After deploying your [Load Balanced Web Services](../concepts/services.en.md#load-balanced-web-service), you should be able to access them publicly via

```
${SvcName}.${EnvName}.${AppName}.${DomainName}
```

For example:

```
https:kudo.test.coolapp.example.aws
```

## How do I configure an alias for my service?
If you don't like the default domain name Copilot assigns to your service, setting an [alias](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html) for your service is also very easy. You can add it directly to your [manifest's](../manifest/overview.en.md) `alias` section. The following snippet will set an alias to your service.

``` yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
  alias: example.aws
```

!!!info
    1. Using this feature requires your app version to be at least `v1.0.0`. You will be prompted to run [`app upgrade`](../commands/app-upgrade.en.md) first if your app version does not meet the requirement.
    2. Currently, you can only use aliases under the domain you specified when creating the application. We'll make this feature more powerful in the future by allowing you to import certificates and use any aliases!

## What happens under the hood?
Under the hood, Copilot

* creates a hosted zone in your app account for the new app subdomain `${AppName}.${DomainName}`
* creates another hosted zone in your env account for the new env subdomain `${EnvName}.${AppName}.${DomainName}`
* creates and validates an ACM certificate for the env subdomain
* associates the certificate with your HTTPS listener and redirects HTTP traffic to HTTPS
* creates an optional A record for your alias

## What does it look like?

<iframe width="560" height="315" src="https://www.youtube.com/embed/Oyr-n59mVjI" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
