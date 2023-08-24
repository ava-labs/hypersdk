import App from "./App";

import Chains from "./components/Chains";
import Keys from "./components/Keys";

const routes = [
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <Chains /> },
      {
        path: "chains",
        element: <Chains />,
      },
      {
        path: "keys",
        element: <Keys />,
      },
    ],
  },
];

export default routes;
