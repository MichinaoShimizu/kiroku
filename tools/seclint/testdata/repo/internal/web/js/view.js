// Fixture for tools/seclint: every line marked "bad" must be reported, the others must not.
el.innerHTML = s; // bad 2
el.outerHTML = s; // bad 3
el.insertAdjacentHTML("beforeend", s); // bad 4
document.write(s); // bad 5
el.innerHTML += s; // bad 6
el.innerHTML = ""; // clearing is fine
if (el.innerHTML === "x") el.textContent = s; // reading is fine
const h = el.innerHTML;
a.innerHTML = ""; b.innerHTML = s; // bad 10 (once)
document.writeln(s); // bad 11
