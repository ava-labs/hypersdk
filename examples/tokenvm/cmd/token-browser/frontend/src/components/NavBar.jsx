import { DashboardOutlined } from "@ant-design/icons";
import { Layout, Menu } from "antd";
import { Link } from "react-router-dom";
import logo from "../assets/images/logo-universal.png";

const items = [
  {
    label: <Link to={"explorer"}>Explorer</Link>,
    key: "explorer",
    icon: <DashboardOutlined />,
  },
];

const NavBar = () => {
  return (
    <Layout.Header theme="light" style={{ background: "white" }}>
      <div
        className="logo"
        style={{
          float: "left",
          marginRight: "200px",
          padding: "1%",
        }}
      >
        <Link to="/">
          <img src={logo} style={{ width: "50px" }} />
        </Link>
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
