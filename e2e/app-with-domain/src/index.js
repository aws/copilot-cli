// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

let express = require('express');

let app1 = express();
app1.get("/", (req, res) => {
    res.send("This is Copilot express app");
});
app1.get("/me", (req, res) => {
    res.send("Hi I am Copilot");
});
app1.listen(3000, () => {
    console.log("Started server on 3000");
});


let app2 = express();
app2.get("/admin", (req, res) => {
    res.send("This is Copilot express app for admin");
});
app2.get("/admin/me", (req, res) => {
    res.send("Hi I am Copilot Admin");
});
app2.listen(3002, () => {
    console.log("Started server on 3002");
});