

function getTerminalRowInd() {
    let terminalRow = -1;
    const rows = document.getElementById("dcf-rows").querySelectorAll("tr");

    rows.forEach((row, index) => {
        const inputs = row.querySelectorAll('input');
        let hasValue = false;
        inputs.forEach(input => {
            if (input.value.trim() !== '') hasValue = true;
        });
        if (hasValue) {
            terminalRow = index;
        }
    });

    return terminalRow;
}


// Helper function to safely extract numbers from formatted text strings
function getCleanNumber(id) {
    const el = document.getElementById(id);
    if (!el || !el.value) return 0;
    // Strip commas, then parse
    return parseFloat(el.value.replace(/,/g, '')) || 0;
};


let futureCashFlows;
let costOfCapitals;
let terminalGrowthRate;

function populateDCFTable() {
    const tbody = document.getElementById('dcf-rows');
    const rows = tbody.querySelectorAll('tr');

    // 1. Grab Year 0 Base Revenue from the global input at the top of the form
    let prevRev = getCleanNumber("input-baseRevenue");
    let shouldBreak = false;
    futureCashFlows = [];
    costOfCapitals = [];

    for(let index = 0; index < rows.length; index++) {
        let row = rows[index];

        // 2. Extract the user's assumptions
        const g = parseFloat(row.querySelector('input[name="revGrowth"]').value) || 0;
        const margin = parseFloat(row.querySelector('input[name="opMargin"]').value) || 0;
        const tax = parseFloat(row.querySelector('input[name="taxRate"]').value) || 0;
        const reinvestInput = parseFloat(row.querySelector('input[name="salesToCapRatio"]').value) || 0;
        const wacc = parseFloat(row.querySelector('input[name="wacc"]').value) || 0;
        costOfCapitals.push(wacc);

        // 3. The Core Math
        const currentRev = prevRev * (1 + g);
        const ebit = currentRev * margin;
        const ebitAfterTax = ebit * (1 - tax);

        let reinvestDollars = 0;
        
        // Handle Terminal Year vs Normal Year logic
        // If the row has the terminal green border, it means reinvestInput is a % Rate, not a Sales/Cap ratio
        if (row.classList.contains('border-green-500') || index === 10) {
            reinvestDollars = ebitAfterTax * reinvestInput;
            terminalGrowthRate = g;
            shouldBreak = true;
        } else {
            if (reinvestInput > 0) {
                reinvestDollars = (currentRev - prevRev) / reinvestInput;
            }
        }

        const fcff = ebitAfterTax - reinvestDollars;
        futureCashFlows.push(fcff);

        // 4. Update the UI with nicely formatted numbers
        const revField = row.querySelector('input[name="revenue"]');
        const ebitField = row.querySelector('input[name="ebit"]');
        const reinvestField = row.querySelector('input[name="reinvest"]');
        const fcffField = row.querySelector('input[name="fcff"]');

        const formatOpts = { minimumFractionDigits: 1, maximumFractionDigits: 1 };

        if(revField) revField.value = currentRev.toLocaleString('en-US', formatOpts);
        if(ebitField) ebitField.value = ebitAfterTax.toLocaleString('en-US', formatOpts); // Often better to show After-Tax EBIT
        if(reinvestField) reinvestField.value = reinvestDollars.toLocaleString('en-US', formatOpts);
        if(fcffField) fcffField.value = fcff.toLocaleString('en-US', formatOpts);

        // 5. Set the current revenue as the "prevRev" for the next row's loop
        prevRev = currentRev;

        if (shouldBreak) break;
    }

    calculateDCFValues();
}

function calculateDCFValues() {
    if(costOfCapitals.length != futureCashFlows.length || costOfCapitals.length === 0) {
        console.log("Wacc and fcf length have to be the same, and greater than 0");
        return;
    }

    let discountRateThusFar = 1.0;
	let PVCashFlows = 0.0;

    // Since costOfCapitals also has the terminal wacc, we calculate the Present Value for only the years upto the terminal year
    for(let i = 0; i < costOfCapitals.length-1; i++) {
        discountRateThusFar *= (1 + costOfCapitals[i])
		PVCashFlows += futureCashFlows[i] / discountRateThusFar
    }

    const terminalValue = futureCashFlows.at(-1)/(costOfCapitals.at(-1) - terminalGrowthRate);
    const PVTerminalValue = terminalValue / discountRateThusFar;
    const valueOfOperatingAssets = PVCashFlows + PVTerminalValue;

    // Get input values!
    const totalCash = getCleanNumber('input-totalCash');
    const totalDebt = getCleanNumber('input-totalDebt');
    const ipoProceeds = getCleanNumber('input-ipoProceeds');
    const nonOpAssets = getCleanNumber('input-nonOpAssets');
    const optionsVal = getCleanNumber('input-optionsValue');
    const totalSharesOutstanding = getCleanNumber('input-sharesOutstanding');

    const equityValue = valueOfOperatingAssets + totalCash + ipoProceeds + nonOpAssets - totalDebt;
    const commonStockEquityValue = equityValue - optionsVal;
    const sharePrice = commonStockEquityValue / totalSharesOutstanding;

    const formatOpts = { minimumFractionDigits: 1, maximumFractionDigits: 1 };

    // Sets UI values
    document.getElementById("res-tv").textContent = "$" + terminalValue.toLocaleString('en-US', formatOpts);
    document.getElementById("res-pv-tv").textContent = "$" + PVTerminalValue.toLocaleString('en-US', formatOpts);
    document.getElementById("res-pv-cf").textContent = "$" + PVCashFlows.toLocaleString('en-US', formatOpts);
    document.getElementById("res-op-assets").textContent = "$" + valueOfOperatingAssets.toLocaleString('en-US', formatOpts);

    document.getElementById("res-debt").textContent = "$" + totalDebt.toLocaleString('en-US', formatOpts);
    document.getElementById("res-cash").textContent = "$" + totalCash.toLocaleString('en-US', formatOpts);
    document.getElementById("res-ipo").textContent = "$" + ipoProceeds.toLocaleString('en-US', formatOpts);
    document.getElementById("res-nonop").textContent = "$" + nonOpAssets.toLocaleString('en-US', formatOpts);

    document.getElementById("res-equity").textContent = "$" + equityValue.toLocaleString('en-US', formatOpts);
    
    document.getElementById("res-options").textContent = "$" + optionsVal.toLocaleString('en-US', formatOpts);
    document.getElementById("res-common-eq").textContent = "$" + commonStockEquityValue.toLocaleString('en-US', formatOpts);

    document.getElementById("res-shares").textContent = totalSharesOutstanding.toLocaleString('en-US', formatOpts);
    
    document.getElementById("res-value-per-share").textContent = "$" + sharePrice.toLocaleString('en-US', formatOpts);
}

function validateDCFTableData() {
    const tbody = document.getElementById('dcf-rows');
    const rows = tbody.querySelectorAll('tr');
    let lastFilledIndex = getTerminalRowInd();

    let isMathValid = true;

    // 2. Apply styles and validate the math on the terminal row
    rows.forEach((row, index) => {
        const labelCell = row.querySelector('.row-label');
        
        row.classList.remove('bg-green-900/30', 'border-l-4', 'border-green-500');
        labelCell.textContent = `Year ${index + 1}`;
        labelCell.classList.remove('text-green-400', 'font-bold', 'text-red-400');
        labelCell.classList.add('text-gray-300');

        if (index === lastFilledIndex) {
            row.classList.add('bg-green-900/30', 'border-l-4', 'border-green-500');
            if(index < 10) {
                labelCell.textContent = `Year ${index + 1} (Terminal)`;
            }
            else {
                labelCell.textContent = `Terminal Year`;
            }
            labelCell.classList.remove('text-gray-300');
            labelCell.classList.add('text-green-400', 'font-bold');

            // MATHEMATICAL VALIDATION
            const termGrowth = parseFloat(row.querySelector('input[name="revGrowth"]').value);
            const termWacc = parseFloat(row.querySelector('input[name="wacc"]').value);
            
            if (!isNaN(termGrowth) && !isNaN(termWacc) && termGrowth >= termWacc) {
                isMathValid = false;
                // Highlight the label in red to show where the error is
                labelCell.classList.remove('text-green-400');
                labelCell.classList.add('text-red-400');
                row.classList.remove('border-green-500');
                row.classList.add('border-red-500');
            }
        }
    });

    // 3. Toggle the UI Error State
    const errorMsg = document.getElementById('dcf-error');

    if (!isMathValid) {
        errorMsg.textContent = "CRITICAL ERROR: Terminal Growth Rate must be less than Terminal WACC.";
        errorMsg.classList.remove('hidden');
    } else {
        errorMsg.classList.add('hidden');
    }
}