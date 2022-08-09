<div class="separator"></div>

<a id="cdn" href="#cdn" class="field">`cdn`</a> <span class="type">Boolean or Map</span>  
The cdn section contains parameters related to integrating your service with a CloudFront distribution.

To enable the CloudFront distribution, specify `cdn: true`.

<span class="parent-field">cdn.</span><a id="cdn-certificate" href="#cdn-certificate" class="field">`certificate`</a> <span class="type">String</span>  
A certificate by which to enable HTTPS traffic on a CloudFront distribution.
CloudFront requires imported certificates to be in the us-east-1 region.