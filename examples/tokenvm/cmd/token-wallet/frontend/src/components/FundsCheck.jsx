import { Alert } from "antd";
import { Link } from "react-router-dom";

const FundsCheck = () => {
  return (
    <>
      {!window.HasBalance &&
        <div style={{margin: "0 0 8px 0"}}>
          <Alert
            message="Warning Text"
            description={
              <div>
                Warning Description Warning Description Warning Description <Link to="/faucet">Warning</Link> Description
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
