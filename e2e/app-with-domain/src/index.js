let express = require('express');

let app1 = express();
let app2 = express();

app1.get("/", (req, res) => {
    res.send("This is Copilot express app");
});

app1.get("/me", (req, res) => {
    res.send("Hi I am Copilot");
});

app2.get("/admin", (req, res) => {
    res.send("This is Copilot express app for admin");
});

app2.get("/admin/me", (req, res) => {
    res.send("Hi I am Copilot Admin");
});

app1.listen(3000, () => {
    console.log("Started server on 3000");
});

app2.listen(3002, () => {
    console.log("Started server on 3002");
});
