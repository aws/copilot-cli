// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

'use strict';

const express = require('express');
const AWS = require('aws-sdk');

// Constants
const PORT = 80;

// Create db connection pool to the Aurora cluster.
const secret = process.env.MYCLUSTER_SECRET
const secretJSON = JSON.parse(secret)
const Pool = require('pg').Pool
const pool = new Pool({
    user: secretJSON['username'],
    host: secretJSON['host'],
    database: secretJSON["dbname"],
    password: secretJSON['password'],
    port: secretJSON['port'],
})

// Get bucket name from environment variable.
const s3 = new AWS.S3({apiVersion: '2006-03-01'});
const bucketName = process.env.MYBUCKET_NAME

// App
const app = express();
app.get('/', (req, res) => {
    res.status(200).send('Hello World');
});

app.get('/databases/:dbName', function(req, res, next) {
    const dbName = req.params['dbName']
    pool.query('SELECT datname FROM pg_database', (error, results) => {
        if (error) {
            res.send(error)
        }
        for (const [idx, row] of results['rows'].entries()) {
            if (row['datname'] === dbName) {
                res.status(200).send("succeed")
                return
            }
        }
        res.status(404)
    })
})

app.get('/peeks3', function(req, res, next) {
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