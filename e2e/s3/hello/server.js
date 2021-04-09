'use strict';

const express = require('express');
const AWS = require('aws-sdk');
const s3 = new AWS.S3({apiVersion: '2006-03-01'});
const bucketName = process.env.MYSTORAGE_NAME

// Constants
const PORT = 80;

// App
const app = express();
app.get('/', (req, res) => {
    res.status(200).send('Hello s3');
});

app.get('/peek', function(req, res, next) {
    let bucketParams = {
        Bucket: bucketName,
    }

    s3.listObjects(bucketParams, function(err, data) {
        if (err) {
            res.status(400).send(err);
            return;
        }

        res.status(200).json({'name': data.Name});
    })
})

app.listen(PORT);
console.log(`Running on http://0.0.0.0:${PORT}`)