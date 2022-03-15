# Domain

!!!attention
    Today, a Route 53 domain name can be associated only when running `copilot app init`.  
    If you'd like to update your application with a domain ([#3045](https://github.com/aws/copilot-cli/issues/3045)), 
    you'll need to initialize a duplicate app with `--domain` and then run `copilot app delete` to 
    remove the old one.

## Load Balanced Web Service
As mentioned in the [Application Guide](../concepts/applications.en.md#additional-app-configurations), you can configure the domain name of your app when running `copilot app init`. After deploying your [Load Balanced Web Services](../concepts/services.en.md#load-balanced-web-service), you should be able to access them publicly via

```
${SvcName}.${EnvName}.${AppName}.${DomainName}
```

if you are using an Application Load Balancer, or

```
${SvcName}-nlb.${EnvName}.${AppName}.${DomainName}
```

if you are using a Network Load Balancer.

For example, `https://kudo.test.coolapp.example.aws` or `kudo-nlb.test.coolapp.example.aws:443`.


Currently, you can only use aliases under the domain you specified when creating the application. Since we [delegate responsibility for the subdomain to Route 53](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/CreatingNewSubdomain.html#UpdateDNSParentDomain), the alias you specify must be in one of these three hosted zones:

- root: `${DomainName}`
- app: `${AppName}.${DomainName}`
- env: `${EnvName}.${AppName}.${DomainName}`

We'll make this feature more powerful in the future by allowing you to import certificates and use any aliases!

## How do I configure an alias for my service?
If you don't like the default domain name Copilot assigns to your service, setting an [alias](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html) for your service is also very easy. You can add it directly to your [manifest's](../manifest/overview.en.md) `alias` section. 
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

## What happens under the hood?
Under the hood, Copilot

* creates a hosted zone in your app account for the new app subdomain `${AppName}.${DomainName}`
* creates another hosted zone in your env account for the new env subdomain `${EnvName}.${AppName}.${DomainName}`
* creates and validates an ACM certificate for the env subdomain
* associates the certificate with:
    - Your HTTPS listener and redirects HTTP traffic to HTTPS, if the alias is used for the Application Load Balancer (`http.alias`)
    - Your network load balancer's TLS listener, if the alias is for `nlb.alias` and TLS termination is enabled.
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

!!!info
    For now, we support only 1-level subdomain such as `web.example.aws`. 
    
    Environment-level domains (e.g. `web.${envName}.${appName}.example.aws`), application-level domains (e.g. `web.${appName}.example.aws`),
    or root domains (i.e. `example.aws`) are not supported yet. This also means that your subdomain shouldn't collide with your application name.

Under the hood, Copilot:

* associates the domain with your app runner service
* creates the domain record as well as the validation records in your root domain's hosted zone
