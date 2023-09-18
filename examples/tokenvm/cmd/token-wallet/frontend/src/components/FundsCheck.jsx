import { Alert } from "antd";
import { Link } from "react-router-dom";

const FundsCheck = () => {
  return (
    <>
      {!window.HasBalance &&
        <div style={{margin: "0 0 8px 0"}}>
          <Alert
            message="No Funds"
            description={
              <div>
                TODO....go to <Link to="/faucet">Faucet</Link>
              </div>
            }
            type="warning"
          />
        </div>
      }
    </>
  )
}

export default FundsCheck;
