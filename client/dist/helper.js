// Define the charset: 0-9, A-Z, a-z, !, @, -
const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!@-";
function genId(length) {
    let result = "";
    const charsetLength = charset.length;
    for (let i = 0; i < length; i++) {
        const randomIndex = Math.floor(Math.random() * charsetLength);
        result += charset.charAt(randomIndex);
    }
    return result;
}
export function gen12p3() {
    const part1 = genId(12);
    const part2 = genId(3);
    return `${part1}-${part2}`;
}
// Example usage:
// console.log(genId(5));
// console.log(get12p3());
//# sourceMappingURL=helper.js.map