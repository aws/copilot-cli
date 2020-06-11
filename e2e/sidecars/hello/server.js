const app = require('koa')();
const router = require('koa-router')();
const lorem = require('lorem-ipsum')

// Log requests
app.use(function *(next){
  const start = new Date;
  yield next;
  const ms = new Date - start;
  console.log('%s %s - %s', this.method, this.url, ms);
});

router.get('/api/lorem-ipsum', function *() {
  this.body = {
    body: lorem({
      count: 10,
      units: 'paragraphs'
    })
  };
});

router.get('/api/health-check', function *() {
  this.body = 'Ready';
});

app.use(router.routes());
app.use(router.allowedMethods());

app.listen(3000);

console.log('Application ready and running...');
