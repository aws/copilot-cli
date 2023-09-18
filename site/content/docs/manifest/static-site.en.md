List of all available properties for a `'Static Site Service'` manifest.

???+ note "Sample static site manifest"

    ```yaml
    name: example
    type: Static Site

    http:
      alias: 'example.com'

    files:
      - source: src/someDirectory
        recursive: true
      - source: someFile.html
    
    # You can override any of the values defined above by environment.
    # environments:
    #   test:
    #     files:
    #       - source: './blob'
    #         destination: 'assets'
    #         recursive: true
    #         exclude: '*'
    #         reinclude:
    #           - '*.txt'
    #           - '*.png'
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. A [Static Site](../concepts/services.en.md#static-site) is an internet-facing service that is hosted by Amazon S3.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
Configuration for incoming traffic to your site.

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String</span>  
HTTPS domain alias of your service.

<span class="parent-field">http.</span><a id="http-certificate" href="#http-certificate" class="field">`certificate`</a> <span class="type">String</span>  
The ARN for the certificate used for your HTTPS traffic.
CloudFront requires imported certificates to be in the `us-east-1` region. For example:

```yaml
http:
  alias: example.com
  certificate: "arn:aws:acm:us-east-1:1234567890:certificate/e5a6e114-b022-45b1-9339-38fbfd6db3e2"
```

<div class="separator"></div>

<a id="files" href="#files" class="field">`files`</a> <span class="type">Array of Maps</span>  
Parameters related to your static assets.

<span class="parent-field">files.</span><a id="files-source" href="#files-source" class="field">`source`</a> <span class="type">String</span>  
The path, relative to your workspace root, to the directory or file to upload to S3.

<span class="parent-field">files.</span><a id="files-recursive" href="#files-recursive" class="field">`recursive`</a> <span class="type">Boolean</span>  
Whether or not the source directory should be uploaded recursively. Defaults to true for directories.

<span class="parent-field">files.</span><a id="files-destination" href="#files-destination" class="field">`destination`</a> <span class="type">String</span>  
Optional. The subpath to be prepended to your files in your S3 bucket. Default value is `.`

<span class="parent-field">files.</span><a id="files-exclude" href="#files-exclude" class="field">`exclude`</a> <span class="type">String</span>  
Optional. Pattern-matched [filters](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3/index.html#use-of-exclude-and-include-filters) to exclude files from upload. Acceptable symbols are:  
`*` (matches everything)  
`?` (matches any single character)  
`[sequence]` (matches any character in `sequence`)  
`[!sequence]` (matches any character not in `sequence`)  

<span class="parent-field">files.</span><a id="files-reinclude" href="#files-reinclude" class="field">`reinclude`</a> <span class="type">String</span>  
Optional. Pattern-matched [filters](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3/index.html#use-of-exclude-and-include-filters) to reinclude files that have been excluded from upload via [`exclude`](#files-exclude). Acceptable symbols are:  
`*` (matches everything)  
`?` (matches any single character)  
`[sequence]` (matches any character in `sequence`)  
`[!sequence]` (matches any character not in `sequence`)  
