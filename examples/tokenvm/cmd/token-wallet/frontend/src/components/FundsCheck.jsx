import { Alert } from "antd";
import { Link } from "react-router-dom";

const FundsCheck = () => {
  return (
    <>
      {!window.HasBalance &&
        <Alert
          message="Warning Text"
          description={
            <div>
              Warning Description Warning Description Warning Description <Link to="/faucet">Warning</Link> Description
            </div>
          }
          type="warning"
        />
      }
    </>
  )
}

export default FundsCheck;
