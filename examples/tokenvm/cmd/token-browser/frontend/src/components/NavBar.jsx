import {useEffect, useState} from "react";
import { GetBalance } from "../../wailsjs/go/main/App";
import { DashboardOutlined, BankOutlined, SendOutlined, ThunderboltOutlined } from "@ant-design/icons";
import { Layout, Menu, Typography } from "antd";
const { Text } = Typography;
import { Link } from "react-router-dom";
import logo from "../assets/images/logo-universal.jpeg";

const items = [
  {
    label: <Link to={"explorer"}>Explorer</Link>,
    key: "explorer",
    icon: <DashboardOutlined />,
  },
  {
    label: <Link to={"mint"}>Mint</Link>,
    key: "mint",
    icon: <BankOutlined />,
  },
  {
    label: <Link to={"transfer"}>Transfer</Link>,
    key: "transfer",
    icon: <SendOutlined />,
  },
  {
    label: <Link to={"faucet"}>Faucet</Link>,
    key: "faucet",
    icon: <ThunderboltOutlined />,
  },
];

const NavBar = () => {
  const [balance, setBalance] = useState("");

  useEffect(() => {
    const getBalance = async () => {
        GetBalance("11111111111111111111111111111111LpoYY")
            .then((balance) => {
                setBalance(balance);
            })
            .catch((error) => {
                messageApi.open({
                    type: "error", content: error,
                });
            });
    };

    getBalance();
    const interval = setInterval(() => {
      getBalance();
    }, 500);

    return () => clearInterval(interval);
  }, []);

  return (
    <Layout.Header theme="light" style={{ background: "white" }}>
      <div
        className="logo"
        style={{ float: "left", padding: "1%" }}
      >
        <img src={logo} style={{ width: "50px" }} />
      </div>
      {/* compute to string represenation */}
      <div style={{ float: "right" }}>
        <Text>{balance} TKN</Text>
      </div>
      <Menu
        defaultSelectedKeys={["explorer"]}
        mode="horizontal"
        items={items}
        style={{
          position: "relative",
        }}
      />
    </Layout.Header>
  );
};

export default NavBar;
