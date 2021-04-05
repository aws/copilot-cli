'use strict';

const express = require('express');
const AWS = require('aws-sdk');

// Constants
const PORT = 80;

// Create db connection pool
const secret = process.env.MYSTORAGE_SECRET
const secretJSON = JSON.parse(secret)
const Pool = require('pg').Pool
const pool = new Pool({
    user: secretJSON['username'],
    host: secretJSON['host'],
    database: secretJSON["dbname"],
    password: secretJSON['password'],
    port: secretJSON['port'],
})

// App
const app = express();
app.get('/', (req, res) => {
    res.send('Hello World');
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

app.listen(PORT);
console.log(`Running on http://0.0.0.0:${PORT}`)